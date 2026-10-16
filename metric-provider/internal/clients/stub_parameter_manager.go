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

package clients

import (
	"context"

	parametermanagerpb "cloud.google.com/go/parametermanager/apiv1/parametermanagerpb"
)

// StubParameterManagerClient is a stub implementation of the parameterManagerClient interface for testing.
type StubParameterManagerClient struct {
	ParamVersion *parametermanagerpb.ParameterVersion
	Err          error
	LastRequest  *parametermanagerpb.GetParameterVersionRequest
}

// GetParameterVersion returns the pre-configured parameter version and error.
func (s *StubParameterManagerClient) GetParameterVersion(ctx context.Context, req *parametermanagerpb.GetParameterVersionRequest) (*parametermanagerpb.ParameterVersion, error) {
	s.LastRequest = req
	return s.ParamVersion, s.Err
}
