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

package orchestrator

import (
	"context"

	"crema/metric-provider/api"
	"crema/metric-provider/internal/scaling"
	pb "crema/metric-provider/proto"

	"github.com/go-logr/logr"
	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"
	"github.com/kedacore/keda/v2/pkg/scaling/cache"
)

type ScalerClient interface {
	Scale(ctx context.Context, req *pb.ScaleRequest) (*pb.ScaleResponse, error)
	Close() error
}

type BuilderFactory interface {
	MakeBuilders(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, asMetricSource bool) ([]cache.ScalerBuilder, error)
}

type StateProvider interface {
	GetScaledObjectState(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, scalerBuilders []cache.ScalerBuilder) (scaling.ScaledObjectState, error)
}

// The orchestrator is responsible for retrieving configuration, using it to fetch metrics, and sending the metrics to the scaling system.
type Orchestrator struct {
	scalerClient   ScalerClient
	cremaConfig    *api.CremaConfig
	builderFactory BuilderFactory
	stateProvider  StateProvider
	logger         *logr.Logger
}

// Create a new Orchestrator. The zero value is not usable.
func New(
	scalerClient ScalerClient,
	cremaConfig *api.CremaConfig,
	builderFactory BuilderFactory,
	stateProvider StateProvider,
	logger *logr.Logger,
) *Orchestrator {
	return &Orchestrator{
		scalerClient:   scalerClient,
		cremaConfig:    cremaConfig,
		builderFactory: builderFactory,
		stateProvider:  stateProvider,
		logger:         logger,
	}
}

// RefreshMetrics fetches metrics for all scaled objects and sends them in a single request to Scaler
func (o *Orchestrator) RefreshMetrics(ctx context.Context) error {
	o.logger.Info("Starting metric collection cycle")

	kedaScaledObjects := scaling.ToKedaScaledObjects(o.cremaConfig)
	triggerAuthentications := o.cremaConfig.Spec.TriggerAuthentications

	var scaledObjectMetrics []*pb.ScaledObjectMetrics

	for _, kedaScaledObject := range kedaScaledObjects {
		logger := o.logger.WithValues("scaleTargetName", kedaScaledObject.Spec.ScaleTargetRef.Name)
		builders, err := o.builderFactory.MakeBuilders(ctx, &kedaScaledObject, triggerAuthentications /*asMetricSource=*/, true)

		// Log errors here rather than returning as we may still be able to retrieve metrics and scale other scaled objects
		if err != nil {
			logger.Error(err, "Unable to refresh metrics")
			continue
		}

		scaledObjectState, err := o.stateProvider.GetScaledObjectState(ctx, &kedaScaledObject, builders)
		if err != nil {
			o.logger.Error(err, "Unable to refresh metrics")
			continue
		}

		scaledObjectMetrics = append(scaledObjectMetrics, &pb.ScaledObjectMetrics{
			ScaledObject: scaling.ToPbScaledObject(kedaScaledObject.Spec),
			Metrics:      toMetrics(scaledObjectState),
		})
	}

	if len(scaledObjectMetrics) > 0 {
		scaleRequest := &pb.ScaleRequest{
			ScaledObjectMetrics: scaledObjectMetrics,
		}
		o.logger.Info("Sending scale request", "scaleRequest", scaleRequest)
		response, err := o.scalerClient.Scale(ctx, scaleRequest)
		if err != nil {
			o.logger.Error(err, "Failed to send scale request")
			return err
		}
		o.logger.Info("Received scale response", "response", response)
	} else {
		o.logger.Info("No objects to scale; skipping scale request")
	}
	return nil
}

func toMetrics(scaledObjectState scaling.ScaledObjectState) []*pb.Metric {
	var pbMetrics []*pb.Metric
	for _, metricAndTargetValue := range scaledObjectState.MetricAndTargetValues {
		metricValue := metricAndTargetValue.MetricValue
		pbMetric := &pb.Metric{
			Value:       metricValue,
			TriggerId:   metricAndTargetValue.TriggerName,
			TriggerType: metricAndTargetValue.TriggerType,
		}
		scaling.PopulateTargetValue(metricAndTargetValue.TargetValue, pbMetric)
		pbMetrics = append(pbMetrics, pbMetric)
	}
	return pbMetrics
}
