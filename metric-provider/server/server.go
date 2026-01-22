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

package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"

	"crema/metric-provider/internal/clients"
	"crema/metric-provider/internal/configprovider"
	"crema/metric-provider/internal/orchestrator"
	"crema/metric-provider/internal/resolvers"
	"crema/metric-provider/internal/scaling"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	port                     = "8080"
	scalerAddress            = "localhost:50051"
	cremaConfigEnvVar        = "CREMA_CONFIG"
	defaultGlobalHttpTimeout = 30 * time.Second
	shutdownTimeout          = 5 * time.Second
)

type Orchestrator interface {
	RefreshMetrics(ctx context.Context) error
}

type ScalerServerClient interface {
	Close() error
}

var parameterVersionRegex = regexp.MustCompile(`^projects/[^/]+/(locations/([^/]+)/)?parameters/[^/]+/versions/[^/]+$`)

// Server wires up all components and handling incoming http requests.
type Server struct {
	scalingOrchestrator Orchestrator
	scalerServerClient  ScalerServerClient
	pollingInterval     *time.Duration // ptr to make it nil-able
	logger              *logr.Logger
}

// Returns a Server. The zero value is not usable.
func New(logger *logr.Logger) (*Server, error) {
	ctx := context.Background()

	log.SetLogger(*logger)

	configID := os.Getenv(cremaConfigEnvVar)
	if configID == "" {
		return nil, fmt.Errorf("environment variable %s is not set", cremaConfigEnvVar)
	}

	cloudRunMetadataClient := clients.CloudRunMetadata()
	projectID, err := cloudRunMetadataClient.GetProjectID()
	if err != nil {
		return nil, fmt.Errorf("failed to get project ID from metadata client: %w", err)
	}

	secretManagerClient, err := clients.SecretManager(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create secret manager client: %w", err)
	}

	scalerServerClient, err := clients.ScalerServer(scalerAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create scaler client: %w", err)
	}

	matches := parameterVersionRegex.FindStringSubmatch(configID)
	if matches == nil {
		return nil, fmt.Errorf("parameter version name %q is not well-formed", configID)
	}
	region := matches[2]
	var pmClient configprovider.ParameterManagerClient
	if region != "" && region != "global" {
		pmClient, err = clients.RegionalParameterManager(ctx, region)
	} else {
		pmClient, err = clients.GlobalParameterManager(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create parameter manager client: %w", err)
	}

	configProvider := configprovider.New(pmClient, configID, logger)
	cremaConfig, err := configProvider.GetCremaConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read crema config: %w", err)
	}

	var pollingInterval *time.Duration
	if cremaConfig.Spec.PollingIntervalSeconds != nil {
		val := time.Duration(*cremaConfig.Spec.PollingIntervalSeconds) * time.Second
		pollingInterval = &val
	}

	authResolver := resolvers.NewAuthResolver(secretManagerClient)
	builderFactory := scaling.NewBuilderFactory(authResolver, defaultGlobalHttpTimeout, logger)
	stateProvider := scaling.NewStateProvider(logger)
	orchestrator := orchestrator.New(scalerServerClient, &cremaConfig, builderFactory, stateProvider, logger)

	return &Server{
		scalingOrchestrator: orchestrator,
		scalerServerClient:  scalerServerClient,
		pollingInterval:     pollingInterval,
		logger:              logger,
	}, nil
}

// Start goroutines for the http server and metric polling (if configured)
func (s *Server) Start(ctx context.Context) error {
	defer s.scalerServerClient.Close()

	var wg sync.WaitGroup

	s.logger.Info("Starting server")

	if s.pollingInterval != nil {
		s.logger.Info("Starting metric polling", "interval", s.pollingInterval.String())
		// Wait until we poling stops to gracefully shutdown
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(*s.pollingInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					if err := s.scalingOrchestrator.RefreshMetrics(ctx); err != nil {
						s.logger.Error(err, "Failed to refresh metrics")
					}
				case <-ctx.Done():
					s.logger.Info("Polling stopped")
					return
				}
			}
		}()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)
	mux.HandleFunc("/healthz", s.handleHealthCheck)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("Listening on port", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error(err, "Failed to start HTTP server")
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	s.logger.Info("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		s.logger.Error(err, "Server shutdown failed")
	} else {
		s.logger.Info("Server gracefully stopped")
	}

	wg.Wait()
	return nil
}

func (s *Server) handleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := s.scalingOrchestrator.RefreshMetrics(r.Context()); err != nil {
		s.logger.Error(err, "Failed to refresh metrics")
		http.Error(w, "failed to refresh metrics", http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "ok")
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "ok")
}
