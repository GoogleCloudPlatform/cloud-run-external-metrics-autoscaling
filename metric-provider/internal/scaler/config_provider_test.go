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

import "testing"

type fakeEnvProvider struct {
	envVars map[string]string
}

func (f fakeEnvProvider) GetEnv(name string) string {
	return f.envVars[name]
}

func TestConfigurationProvider(t *testing.T) {
	t.Run("staticConfig_withoutUseMinInstancesFlag_returnsUseMinInstancesFalse", func(t *testing.T) {
		cp := NewConfigurationProvider(fakeEnvProvider{envVars: map[string]string{}})
		cfg := cp.StaticConfig()
		if cfg.UseMinInstances {
			t.Errorf("got UseMinInstances = true, want false")
		}
	})

	t.Run("staticConfig_withUseMinInstancesFlagFalse_useMinInstancesFalse", func(t *testing.T) {
		cp := NewConfigurationProvider(fakeEnvProvider{envVars: map[string]string{"USE_MIN_INSTANCES": "false"}})
		cfg := cp.StaticConfig()
		if cfg.UseMinInstances {
			t.Errorf("got UseMinInstances = true, want false")
		}
	})

	t.Run("staticConfig_withUseMinInstancesFlagTrue_useMinInstancesTrue", func(t *testing.T) {
		cp := NewConfigurationProvider(fakeEnvProvider{envVars: map[string]string{"USE_MIN_INSTANCES": "true"}})
		cfg := cp.StaticConfig()
		if !cfg.UseMinInstances {
			t.Errorf("got UseMinInstances = false, want true")
		}
	})

	t.Run("staticConfig_withoutOutputScalerMetricsFlag_returnsOutputScalerMetricsFalse", func(t *testing.T) {
		cp := NewConfigurationProvider(fakeEnvProvider{envVars: map[string]string{}})
		cfg := cp.StaticConfig()
		if cfg.OutputScalerMetrics {
			t.Errorf("got OutputScalerMetrics = true, want false")
		}
	})

	t.Run("staticConfig_withOutputScalerMetricsFlag_returnsOutputScalerMetricsTrue", func(t *testing.T) {
		cp := NewConfigurationProvider(fakeEnvProvider{envVars: map[string]string{"OUTPUT_SCALER_METRICS": "TRUe"}})
		cfg := cp.StaticConfig()
		if !cfg.OutputScalerMetrics {
			t.Errorf("got OutputScalerMetrics = false, want true")
		}
	})

	t.Run("newConfigurationProvider_withNilEnvProvider_panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic for nil envProvider")
			}
		}()
		NewConfigurationProvider(nil)
	})
}
