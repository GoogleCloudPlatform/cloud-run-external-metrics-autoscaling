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
	"testing"

	"sigs.k8s.io/yaml"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr bool
	}{
    {
			name: "Valid config",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers:
          - type: cpu
            metadata:
              value: "50"
  triggerAuthentications:
    - metadata:
        name: my-auth
      spec:
        podIdentity:
          provider: gcp
`,
			wantErr: false,
		},
    {
			name: "Fails on missing auth definition",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers:
          - type: cpu
            metadata:
              value: "50"
            authenticationRef:
              name: missing-auth
  triggerAuthentications:
    - metadata:
        name: other-auth
      spec:
        gcpSecretManager:
          secrets:
            - parameter: foo
              id: bar
              version: baz
`,
			wantErr: true,
		},
		{
			name: "Fails on empty scaled objects",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects: []
`,
			wantErr: true,
		},
		{
			name: "Fails on scaled object missing scaleTargetRef",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        triggers:
          - type: cpu
            metadata:
              value: "50"
`,
			wantErr: true,
		},
		{
			name: "Fails on scaled object missing scaleTargetRef name",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: ""
        triggers:
          - type: cpu
            metadata:
              value: "50"
`,
			wantErr: true,
		},
		{
			name: "Fails on scaled object missing triggers",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers: []
`,
			wantErr: true,
		},
		{
			name: "Fails on trigger missing type",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers:
          - type: ""
            metadata:
              value: "50"
`,
			wantErr: true,
		},
		{
			name: "Fails on trigger authentication missing name",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers:
          - type: cpu
            metadata:
              value: "50"
  triggerAuthentications:
    - spec:
        gcpSecretManager:
          secrets:
            - parameter: foo
              id: bar
              version: baz
`,
			wantErr: true,
		},
		{
			name: "Fails on trigger authentication missing spec",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers:
          - type: cpu
            metadata:
              value: "50"
  triggerAuthentications:
    - metadata:
        name: my-auth
      spec: {}
`,
			wantErr: true,
		},
		{
			name: "Fails on trigger authentication podIdentity missing provider",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers:
          - type: cpu
            metadata:
              value: "50"
  triggerAuthentications:
    - metadata:
        name: my-auth
      spec:
        podIdentity: {}
`,
			wantErr: true,
		},
		{
			name: "Fails on scaled object with pollingInterval",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        pollingInterval: 30
        scaleTargetRef:
          name: my-service
        triggers:
          - type: cpu
            metadata:
              value: "50"
`,
			wantErr: true,
		},
		{
			name: "Fails on duplicate trigger authentication names",
			config: `
apiVersion: crema/v1
kind: CremaConfig
spec:
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: my-service
        triggers:
          - type: cpu
            metadata:
              value: "50"
  triggerAuthentications:
    - metadata:
        name: same-name
      spec:
        podIdentity:
          provider: gcp
    - metadata:
        name: same-name
      spec:
        podIdentity:
          provider: gcp
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config api.CremaConfig
			if err := yaml.Unmarshal([]byte(tt.config), &config); err != nil {
				t.Fatalf("failed to unmarshal test config: %v", err)
			}
			if err := ValidateConfig(config); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}