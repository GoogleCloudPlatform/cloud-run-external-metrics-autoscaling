/*
 * Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package com.google.cloud.run.crema;

import static com.google.common.truth.Truth.assertThat;

import com.google.common.collect.ImmutableMap;
import java.util.Map;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.junit.runners.JUnit4;

@RunWith(JUnit4.class)
public final class ConfigurationProviderTest {
  private static class FakeEnvProvider implements ConfigurationProvider.EnvProvider {
    private final Map<String, String> envVars;

    FakeEnvProvider(Map<String, String> envVars) {
      this.envVars = envVars;
    }

    @Override
    public String getEnv(String name) {
      return envVars.get(name);
    }
  }

  @Test
  public void staticConfig_withoutUseMinInstancesFlag_returnsUseMinInstancesFalse() {
    ConfigurationProvider configuration =
        new ConfigurationProvider(new FakeEnvProvider(ImmutableMap.of()));

    ConfigurationProvider.StaticConfig staticConfig = configuration.staticConfig();
    assertThat(staticConfig.useMinInstances()).isFalse();
  }

  @Test
  public void staticConfig_withUseMinInstancesFlagFalse_useMinInstancesFalse() {
    ConfigurationProvider configuration =
        new ConfigurationProvider(
            new FakeEnvProvider(ImmutableMap.of("USE_MIN_INSTANCES", "false")));

    ConfigurationProvider.StaticConfig staticConfig = configuration.staticConfig();
    assertThat(staticConfig.useMinInstances()).isFalse();
  }

  @Test
  public void staticConfig_withUseMinInstancesFlagTrue_useMinInstancesTrue() {
    ConfigurationProvider configuration =
        new ConfigurationProvider(
            new FakeEnvProvider(ImmutableMap.of("USE_MIN_INSTANCES", "true")));

    ConfigurationProvider.StaticConfig staticConfig = configuration.staticConfig();
    assertThat(staticConfig.useMinInstances()).isTrue();
  }

  @Test
  public void staticConfig_withoutOutputScalerMetricsFlag_returnsOutputScalerMetricsFalse() {
    ConfigurationProvider configuration =
        new ConfigurationProvider(new FakeEnvProvider(ImmutableMap.of()));

    ConfigurationProvider.StaticConfig staticConfig = configuration.staticConfig();
    assertThat(staticConfig.outputScalerMetrics()).isFalse();
  }

  @Test
  public void staticConfig_withOutputScalerMetricsFlag_returnsOutputScalerMetricsTrue() {
    ConfigurationProvider configuration =
        new ConfigurationProvider(
            new FakeEnvProvider(ImmutableMap.of("OUTPUT_SCALER_METRICS", "TRUe")));

    ConfigurationProvider.StaticConfig staticConfig = configuration.staticConfig();
    assertThat(staticConfig.outputScalerMetrics()).isTrue();
  }
}
