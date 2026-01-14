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

package config

import (
	"crema/metric-provider/api"
	"fmt"
)

// ValidateConfig checks if an unmarshalled CremaConfig is valid.
func ValidateConfig(config api.CremaConfig) error {
	if len(config.Spec.ScaledObjects) == 0 {
		return fmt.Errorf("spec.scaledObjects: must have at least one scaled object")
	}

	auths := make(map[string]bool)
	for i, ta := range config.Spec.TriggerAuthentications {
		if err := validateTriggerAuthentication(ta, i); err != nil {
			return err
		}
		if auths[ta.Name] {
			return fmt.Errorf("triggerAuthentications[%d].metadata.name: must be unique", i)
		}
		auths[ta.Name] = true
	}

	for i, so := range config.Spec.ScaledObjects {
		if err := validateScaledObject(so, auths, i); err != nil {
			return err
		}
	}

	return nil
}

func validateTriggerAuthentication(ta api.TriggerAuthentication, index int) error {
	if ta.Name == "" {
		return fmt.Errorf("triggerAuthentications[%d].metadata.name: must be set", index)
	}

	if ta.Spec.PodIdentity == nil && ta.Spec.GCPSecretManager == nil {
		return fmt.Errorf("triggerAuthentications[%d].spec: must be set", index)
	}

	if ta.Spec.PodIdentity != nil {
		if ta.Spec.PodIdentity.Provider == "" {
			return fmt.Errorf("triggerAuthentications[%d].spec.podIdentity.provider: must be set", index)
		}
	}
	return nil
}

func validateScaledObject(so api.CremaScaledObject, auths map[string]bool, index int) error {
	if so.Spec.ScaleTargetRef == nil || so.Spec.ScaleTargetRef.Name == "" {
		return fmt.Errorf("scaledObjects[%d].spec.scaleTargetRef.name: must be set", index)
	}

	// This unsupported field is not caught by UnmarshallStrict because KEDA's ScaledObject spec contains a PollingInterval field.
	if so.Spec.PollingInterval != nil {
		return fmt.Errorf("scaledObjects[%d].spec.pollingInterval: must only be specified as part of top-level CremaConfig.spec", index)
	}

	if len(so.Spec.Triggers) == 0 {
		return fmt.Errorf("scaledObjects[%d].spec.triggers: must have at least one trigger", index)
	}

	for j, trigger := range so.Spec.Triggers {
		if trigger.Type == "" {
			return fmt.Errorf("scaledObjects[%d].spec.triggers[%d].type: must be set", index, j)
		}

		if trigger.AuthenticationRef != nil && trigger.AuthenticationRef.Name != "" {
			if !auths[trigger.AuthenticationRef.Name] {
				return fmt.Errorf("scaledObjects[%d].spec.triggers[%d].authenticationRef.name: trigger authentication %q not found", index, j, trigger.AuthenticationRef.Name)
			}
		}
	}
	return nil
}