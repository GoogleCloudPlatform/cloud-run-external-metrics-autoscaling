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

package scaling

import (
	"fmt"
	"strings"
)

// Metric holds a single trigger's current metric value and target value.
type Metric struct {
	Value              float64
	TargetValue        *float64
	TargetAverageValue *float64
	TriggerID          string
	TriggerType        string
}

func (m *Metric) String() string {
	if m == nil {
		return "<nil>"
	}
	var parts []string
	parts = append(parts, fmt.Sprintf("value:%v", m.Value))
	if m.TargetValue != nil {
		parts = append(parts, fmt.Sprintf("target_value:%v", *m.TargetValue))
	}
	if m.TargetAverageValue != nil {
		parts = append(parts, fmt.Sprintf("target_average_value:%v", *m.TargetAverageValue))
	}
	if m.TriggerID != "" {
		parts = append(parts, fmt.Sprintf("trigger_id:%q", m.TriggerID))
	}
	if m.TriggerType != "" {
		parts = append(parts, fmt.Sprintf("trigger_type:%q", m.TriggerType))
	}
	return "{" + strings.Join(parts, " ") + "}"
}

func (m *Metric) HasTargetValue() bool {
	return m != nil && m.TargetValue != nil
}

func (m *Metric) GetTargetValue() float64 {
	if m != nil && m.TargetValue != nil {
		return *m.TargetValue
	}
	return 0
}

func (m *Metric) HasTargetAverageValue() bool {
	return m != nil && m.TargetAverageValue != nil
}

func (m *Metric) GetTargetAverageValue() float64 {
	if m != nil && m.TargetAverageValue != nil {
		return *m.TargetAverageValue
	}
	return 0
}

// ScaledObjectMetrics pairs a ScaledObject with its collected trigger metrics.
type ScaledObjectMetrics struct {
	ScaledObject *ScaledObject
	Metrics      []*Metric
}

func (s *ScaledObjectMetrics) GetScaledObject() *ScaledObject {
	if s != nil {
		return s.ScaledObject
	}
	return nil
}

func (s *ScaledObjectMetrics) String() string {
	if s == nil {
		return "<nil>"
	}
	metrics := make([]string, len(s.Metrics))
	for i, m := range s.Metrics {
		metrics[i] = m.String()
	}
	return fmt.Sprintf("{scaled_object:%s metrics:[%s]}", s.ScaledObject.String(), strings.Join(metrics, " "))
}

// ScalingStatus represents the outcome of scaling a workload.
type ScalingStatus int32

const (
	ScalingStatusUnspecified ScalingStatus = 0
	ScalingStatusSucceeded   ScalingStatus = 1
	ScalingStatusFailed      ScalingStatus = 2
)

func (s ScalingStatus) String() string {
	switch s {
	case ScalingStatusSucceeded:
		return "SUCCEEDED"
	case ScalingStatusFailed:
		return "FAILED"
	default:
		return "SCALING_STATUS_UNSPECIFIED"
	}
}

// ScalingResult represents the result of scaling a single target workload.
type ScalingResult struct {
	ScaleTargetName string
	Status          ScalingStatus
	Message         string
}

func (r *ScalingResult) String() string {
	if r == nil {
		return "<nil>"
	}
	if r.Message != "" {
		return fmt.Sprintf("{scale_target_name:%q status:%s message:%q}", r.ScaleTargetName, r.Status, r.Message)
	}
	return fmt.Sprintf("{scale_target_name:%q status:%s}", r.ScaleTargetName, r.Status)
}

// ScaleRequest holds a batch of ScaledObjectMetrics to scale.
type ScaleRequest struct {
	ScaledObjectMetrics []*ScaledObjectMetrics
}

func (r *ScaleRequest) String() string {
	if r == nil {
		return "<nil>"
	}
	items := make([]string, len(r.ScaledObjectMetrics))
	for i, som := range r.ScaledObjectMetrics {
		items[i] = som.String()
	}
	return fmt.Sprintf("scaled_object_metrics:[%s]", strings.Join(items, " "))
}

// ScaleResponse holds the results for a ScaleRequest batch.
type ScaleResponse struct {
	Results []*ScalingResult
}

func (r *ScaleResponse) String() string {
	if r == nil {
		return "<nil>"
	}
	items := make([]string, len(r.Results))
	for i, res := range r.Results {
		items[i] = res.String()
	}
	return fmt.Sprintf("results:[%s]", strings.Join(items, " "))
}
