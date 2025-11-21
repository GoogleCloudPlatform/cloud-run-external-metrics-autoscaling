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

package logging

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/go-logr/logr"
)

// logSink implements logr.LogSink.
type logSink struct {
	logger *log.Logger
	prefix string
	values []interface{}
}

// NewLogger creates a new logr.Logger that writes to stdout.
func NewLogger() logr.Logger {
	prefix := "[METRIC-PROVIDER]"
	return logr.New(logSink{
		logger: log.New(os.Stdout, "", 0),
		prefix: prefix,
	})
}

// Required for logr interface
func (ls logSink) Init(info logr.RuntimeInfo) {
}

// Required for logr interface
func (ls logSink) Enabled(level int) bool {
	return true
}

func (ls logSink) Info(level int, msg string, keysAndValues ...interface{}) {
	kvs := append(ls.values, keysAndValues...)
	ls.logger.Printf("[INFO] %s %s %s", ls.prefix, msg, ls.formatKVs(kvs))
}

func (ls logSink) Error(err error, msg string, keysAndValues ...interface{}) {
	kvs := append(ls.values, keysAndValues...)
	ls.logger.Printf("[ERROR] %s %s: %v%s", ls.prefix, msg, err, ls.formatKVs(kvs))
}

func (ls logSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	newLogger := ls
	newLogger.values = append(newLogger.values, keysAndValues...)
	return newLogger
}

func (ls logSink) WithName(name string) logr.LogSink {
	newLogger := ls
	if len(ls.prefix) > 0 {
		// Remove brackets
		prefix := ls.prefix[1 : len(ls.prefix)-1]
		newLogger.prefix = fmt.Sprintf("[%s/%s]", prefix, name)
	} else {
		newLogger.prefix = fmt.Sprintf("[%s]", name)
	}
	return newLogger
}

func (ls logSink) formatKVs(keysAndValues []interface{}) string {
	if len(keysAndValues) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(" ")
	for i := 0; i < len(keysAndValues); i += 2 {
		sb.WriteString(fmt.Sprintf("%s=", keysAndValues[i]))
		if i+1 < len(keysAndValues) {
			sb.WriteString(fmt.Sprintf("%+v", keysAndValues[i+1]))
		}
		if i+2 < len(keysAndValues) {
			sb.WriteString(" ")
		}
	}
	return sb.String()
}
