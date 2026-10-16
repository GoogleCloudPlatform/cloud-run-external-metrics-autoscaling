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

package api

import (
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CremaConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CremaConfigSpec `json:"spec"`
}

type CremaConfigSpec struct {
	ScaledObjects []CremaScaledObject `json:"scaledObjects"`

	TriggerAuthentications []TriggerAuthentication `json:"triggerAuthentications"`

	// +optional
	PollingIntervalSeconds *int32 `json:"pollingInterval,omitempty"`
}

type CremaScaledObject struct {
	Spec kedav1alpha1.ScaledObjectSpec `json:"spec"`
	// +optional
	Status kedav1alpha1.ScaledObjectStatus `json:"status,omitempty"`
}

type TriggerAuthentication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec TriggerAuthenticationSpec `json:"spec"`
}

type TriggerAuthenticationSpec struct {
	// +optional
	PodIdentity *AuthPodIdentity `json:"podIdentity,omitempty"`

	// +optional
	GCPSecretManager *kedav1alpha1.GCPSecretManager `json:"gcpSecretManager,omitempty"`
}

type AuthPodIdentity struct {
	Provider PodIdentityProvider `json:"provider"`
}

type PodIdentityProvider string

const (
	PodIdentityProviderNone          PodIdentityProvider = "none"
	PodIdentityProviderGCP           PodIdentityProvider = "gcp"
)