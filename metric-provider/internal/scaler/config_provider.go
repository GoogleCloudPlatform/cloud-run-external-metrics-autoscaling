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
	"os"
	"strings"
)

const (
	outputScalerMetricsDefault = false
	useMinInstancesDefault     = false
)

// StaticConfig holds static configuration for Scaler.
type StaticConfig struct {
	UseMinInstances     bool
	OutputScalerMetrics bool
}

// EnvProvider provides access to environment variables.
type EnvProvider interface {
	GetEnv(name string) string
}

// SystemEnvProvider reads environment variables from the OS environment.
type SystemEnvProvider struct{}

// GetEnv returns the value of the environment variable named by the key.
func (SystemEnvProvider) GetEnv(name string) string {
	return os.Getenv(name)
}

// ConfigurationProvider reads and validates user-provided configuration from environment variables.
type ConfigurationProvider struct {
	envProvider EnvProvider
}

// NewConfigurationProvider creates a new ConfigurationProvider.
func NewConfigurationProvider(envProvider EnvProvider) *ConfigurationProvider {
	if envProvider == nil {
		panic("envProvider cannot be nil")
	}
	return &ConfigurationProvider{envProvider: envProvider}
}

// StaticConfig reads environment variables to create a StaticConfig.
func (c *ConfigurationProvider) StaticConfig() StaticConfig {
	useMinInstances := useMinInstancesDefault
	if val := c.envProvider.GetEnv("USE_MIN_INSTANCES"); val != "" {
		useMinInstances = strings.EqualFold(val, "true")
	}

	outputScalerMetrics := outputScalerMetricsDefault
	if val := c.envProvider.GetEnv("OUTPUT_SCALER_METRICS"); val != "" {
		outputScalerMetrics = strings.EqualFold(val, "true")
	}

	return StaticConfig{
		UseMinInstances:     useMinInstances,
		OutputScalerMetrics: outputScalerMetrics,
	}
}
