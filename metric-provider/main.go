// Copyright 2025 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"crema/metric-provider/internal/logging"
	"crema/metric-provider/server"
)

func main() {
	logger := logging.NewLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		cancel()
	}()

	s, err := server.New(&logger)
	if err != nil {
		logger.Error(err, "Failed to create server")
		os.Exit(1)
	}

	if err := s.Start(ctx); err != nil {
		logger.Error(err, "Server returned an error")
		os.Exit(1)
	}
}
