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

package scaler

import (
	"context"
	"fmt"
	"sync"

	pb "crema/metric-provider/proto"

	"github.com/go-logr/logr"
)

// ScalersManager manages scaler instances for all scaled objects.
type ScalersManager struct {
	mu                    sync.RWMutex
	scalers               map[string]*Scaler
	cloudRunClient        CloudRunClient
	cloudMonitoringClient CloudMonitoringClient
	logger                logr.Logger
}

// NewScalersManager creates a new ScalersManager.
func NewScalersManager(
	cloudRunClient CloudRunClient,
	cloudMonitoringClient CloudMonitoringClient,
	logger *logr.Logger,
) *ScalersManager {
	scalerLogger := logger.WithName("scaler")
	return &ScalersManager{
		scalers:               make(map[string]*Scaler),
		cloudRunClient:        cloudRunClient,
		cloudMonitoringClient: cloudMonitoringClient,
		logger:                scalerLogger,
	}
}

// Scale executes a scale operation for each scaled object in the request concurrently.
func (m *ScalersManager) Scale(ctx context.Context, req *pb.ScaleRequest) (*pb.ScaleResponse, error) {
	m.logger.Info(fmt.Sprintf("Received ScaleRequest: %+v", req))

	var metricsList []*pb.ScaledObjectMetrics
	if req != nil {
		metricsList = req.ScaledObjectMetrics
	}

	results := make([]*pb.ScalingResult, len(metricsList))
	var wg sync.WaitGroup
	wg.Add(len(metricsList))

	for i, som := range metricsList {
		go func(idx int, scaledObjectMetrics *pb.ScaledObjectMetrics) {
			defer wg.Done()
			var scaleTargetName string
			if scaledObjectMetrics != nil && scaledObjectMetrics.ScaledObject != nil && scaledObjectMetrics.ScaledObject.ScaleTargetRef != nil {
				scaleTargetName = scaledObjectMetrics.ScaledObject.ScaleTargetRef.Name
			}
			defer func() {
				if r := recover(); r != nil {
					m.logger.Error(fmt.Errorf("panic: %v", r), fmt.Sprintf("Failed to scale %s", scaleTargetName))
					results[idx] = &pb.ScalingResult{
						ScaleTargetName: scaleTargetName,
						Status:          pb.ScalingStatus_FAILED,
					}
				}
			}()

			scaler, err := m.getOrCreateScaler(scaledObjectMetrics.GetScaledObject())
			if err != nil {
				m.logger.Error(err, fmt.Sprintf("Failed to scale %s", scaleTargetName))
				results[idx] = &pb.ScalingResult{
					ScaleTargetName: scaleTargetName,
					Status:          pb.ScalingStatus_FAILED,
				}
				return
			}

			status, err := scaler.Scale(ctx, scaledObjectMetrics)
			if err != nil {
				m.logger.Error(err, fmt.Sprintf("Failed to scale %s", scaleTargetName))
				results[idx] = &pb.ScalingResult{
					ScaleTargetName: scaleTargetName,
					Status:          pb.ScalingStatus_FAILED,
				}
				return
			}

			results[idx] = &pb.ScalingResult{
				ScaleTargetName: scaleTargetName,
				Status:          status,
			}
		}(i, som)
	}

	wg.Wait()
	return &pb.ScaleResponse{Results: results}, nil
}

// Close closes underlying clients.
func (m *ScalersManager) Close() error {
	if m.cloudMonitoringClient != nil {
		return m.cloudMonitoringClient.Close()
	}
	return nil
}

func (m *ScalersManager) getOrCreateScaler(scaledObject *pb.ScaledObject) (*Scaler, error) {
	var scaleTargetName string
	if scaledObject != nil && scaledObject.ScaleTargetRef != nil {
		scaleTargetName = scaledObject.ScaleTargetRef.Name
	}

	m.mu.RLock()
	existing, ok := m.scalers[scaleTargetName]
	m.mu.RUnlock()
	if ok {
		return existing, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if race, ok := m.scalers[scaleTargetName]; ok {
		return race, nil
	}

	created, err := m.buildScaler(scaledObject)
	if err != nil {
		return nil, err
	}
	m.scalers[scaleTargetName] = created
	m.logger.Info(fmt.Sprintf("Created new scaler for %s", scaleTargetName))
	return created, nil
}

func (m *ScalersManager) buildScaler(scaledObject *pb.ScaledObject) (*Scaler, error) {
	var scaleTargetName string
	if scaledObject != nil && scaledObject.ScaleTargetRef != nil {
		scaleTargetName = scaledObject.ScaleTargetRef.Name
	}
	workloadInfo, err := ParseWorkloadInfo(scaleTargetName)
	if err != nil {
		return nil, err
	}
	staticConfig := NewConfigurationProvider(SystemEnvProvider{}).StaticConfig()
	metricsService, err := NewMetricsService(m.cloudMonitoringClient, workloadInfo.ProjectID, m.logger)
	if err != nil {
		return nil, err
	}
	return NewScaler(m.cloudRunClient, metricsService, staticConfig, workloadInfo.ProjectID, m.logger), nil
}
