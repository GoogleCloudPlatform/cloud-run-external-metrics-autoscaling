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

import static com.google.common.base.Strings.isNullOrEmpty;

import com.google.common.base.Preconditions;

/**
 * Configuration provider for the Scaler.
 *
 * <p>Reads and validates the user-provided configuration from environment variables.
 */
class ConfigurationProvider {
  private static final boolean OUTPUT_SCALER_METRICS_DEFAULT = false;
  private static final boolean USE_MIN_INSTANCES_DEFAULT = false;

  /**
   * Static configuration for Scaler. These fields should only be read at startup, and should not be
   * changed during the lifetime of the application.
   */
  public record StaticConfig(boolean useMinInstances, boolean outputScalerMetrics) {}

  public static interface EnvProvider {
    String getEnv(String name);
  }

  /**
   * Provides access to the system environment.
   *
   * <p>This interface allows mocking the system environment in tests.
   */
  public static class SystemEnvProvider implements ConfigurationProvider.EnvProvider {
    @Override
    public String getEnv(String name) {
      return System.getenv(name);
    }
  }

  private final EnvProvider envProvider;

  public ConfigurationProvider(EnvProvider envProvider) {
    this.envProvider = Preconditions.checkNotNull(envProvider, "Env provider cannot be null.");
  }

  /**
   * Provides scaling configuration for the Scaler.
   *
   * <p>Reads from environment variables to create a {@code StaticConfig} object.
   *
   * @return Scaling configuration.
   */
  StaticConfig staticConfig() {
    boolean useMinInstances = USE_MIN_INSTANCES_DEFAULT;
    if (!isNullOrEmpty(envProvider.getEnv("USE_MIN_INSTANCES"))) {
      useMinInstances = Boolean.parseBoolean(envProvider.getEnv("USE_MIN_INSTANCES"));
    }

    boolean outputScalerMetrics = OUTPUT_SCALER_METRICS_DEFAULT;
    if (!isNullOrEmpty(envProvider.getEnv("OUTPUT_SCALER_METRICS"))) {
      outputScalerMetrics = Boolean.parseBoolean(envProvider.getEnv("OUTPUT_SCALER_METRICS"));
    }

    return new StaticConfig(useMinInstances, outputScalerMetrics);
  }
}
