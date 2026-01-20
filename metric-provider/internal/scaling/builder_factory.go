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
	"context"
	"crema/metric-provider/api"
	"crema/metric-provider/internal/resolvers"

	"fmt"
	"time"

	kedav1alpha1 "github.com/kedacore/keda/v2/apis/keda/v1alpha1"

	"github.com/go-logr/logr"
	"github.com/kedacore/keda/v2/pkg/scalers"
	"github.com/kedacore/keda/v2/pkg/scalers/scalersconfig"
	"github.com/kedacore/keda/v2/pkg/scaling/cache"
)

// BuilderFactory contains the logic for creating ScalerBuilders.
type BuilderFactory struct {
	authResolver      *resolvers.AuthResolver
	globalHTTPTimeout time.Duration
	logger            *logr.Logger
}

// Create a new BuilderFactory instance. The zero value is not usable.
func NewBuilderFactory(authResolver *resolvers.AuthResolver, globalHTTPTimeout time.Duration, logger *logr.Logger) *BuilderFactory {
	return &BuilderFactory{
		authResolver:      authResolver,
		globalHTTPTimeout: globalHTTPTimeout,
		logger:            logger,
	}
}

// MakeBuilders returns a list of ScalerBuilders for the specified triggers.
// This allows the function to attempt to build all scalers and report on the success or failure of each.
func (bf *BuilderFactory) MakeBuilders(ctx context.Context, scaledObject *kedav1alpha1.ScaledObject, triggerAuths []api.TriggerAuthentication, asMetricSource bool) ([]cache.ScalerBuilder, error) {
	logger := bf.logger.WithValues("scaleTargetName", scaledObject.Spec.ScaleTargetRef.Name)
	builders := make([]cache.ScalerBuilder, 0, len(scaledObject.Spec.Triggers))
	resolvedEnv := resolvers.ResolveEnv()

	for i, trigger := range scaledObject.Spec.Triggers {
		perTriggerLogger := logger.WithValues("triggerIndex", i)

		factory := func() (scalers.Scaler, *scalersconfig.ScalerConfig, error) {
			config := &scalersconfig.ScalerConfig{
				ScalableObjectName:      scaledObject.Name,
				ScalableObjectNamespace: scaledObject.Namespace,
				TriggerName:             trigger.Name,
				TriggerMetadata:         trigger.Metadata,
				TriggerType:             trigger.Type,
				TriggerUseCachedMetrics: trigger.UseCachedMetrics,
				ResolvedEnv:             resolvedEnv,
				AuthParams:              make(map[string]string),
				GlobalHTTPTimeout:       bf.globalHTTPTimeout,
				TriggerIndex:            i,
				MetricType:              trigger.MetricType,
				AsMetricSource:          asMetricSource,
				ScaledObject:            scaledObject,
				TriggerUniqueKey:        fmt.Sprintf("%s-%s-%s-%d", scaledObject.Kind, scaledObject.Namespace, scaledObject.Name, i),
			}

			if trigger.AuthenticationRef != nil {
				authParams, podIdentity, err := bf.authResolver.ResolveAuthRefAndPodIdentity(ctx, triggerAuths, trigger.AuthenticationRef.Name)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to resolve auth params for triggerIndex=%d: %w", i, err)
				}
				config.AuthParams = authParams
				config.PodIdentity = podIdentity
			}

			scaler, err := buildScaler(ctx, trigger.Type, config)

			if err != nil {
				if scaler != nil {
					if closeErr := scaler.Close(ctx); closeErr != nil {
						perTriggerLogger.Error(closeErr, "Failed to close scaler")
					}
				}
				return nil, nil, fmt.Errorf("failed to create scaler for triggerIndex=%d: %w", i, err)
			}

			return scaler, config, nil
		}

		scaler, config, err := factory()

		if err != nil {
			logger.Error(err, "Failure while building scalers")
		} else {
			builders = append(builders, cache.ScalerBuilder{
				Scaler:       scaler,
				ScalerConfig: *config,
				Factory:      factory,
			})
		}
	}

	// builders will be empty if we failed to create any scalers
	if len(builders) == 0 {
		return builders, fmt.Errorf("failed to create any scalers for scaledObject")
	}

	return builders, nil
}

// buildScaler builds a scaler form input config and trigger type
func buildScaler(ctx context.Context, triggerType string, config *scalersconfig.ScalerConfig) (scalers.Scaler, error) {
	switch triggerType {
	case "activemq":
		return scalers.NewActiveMQScaler(config)
	case "apache-kafka":
		return scalers.NewApacheKafkaScaler(ctx, config)
	case "arangodb":
		return scalers.NewArangoDBScaler(config)
	case "artemis-queue":
		return scalers.NewArtemisQueueScaler(config)
	case "aws-cloudwatch":
		return scalers.NewAwsCloudwatchScaler(ctx, config)
	case "aws-dynamodb":
		return scalers.NewAwsDynamoDBScaler(ctx, config)
	case "aws-dynamodb-streams":
		return scalers.NewAwsDynamoDBStreamsScaler(ctx, config)
	case "aws-kinesis-stream":
		return scalers.NewAwsKinesisStreamScaler(ctx, config)
	case "aws-sqs-queue":
		return scalers.NewAwsSqsQueueScaler(ctx, config)
	case "azure-app-insights":
		return scalers.NewAzureAppInsightsScaler(config)
	case "azure-blob":
		return scalers.NewAzureBlobScaler(config)
	case "azure-data-explorer":
		return scalers.NewAzureDataExplorerScaler(config)
	case "azure-eventhub":
		return scalers.NewAzureEventHubScaler(config)
	case "azure-log-analytics":
		return scalers.NewAzureLogAnalyticsScaler(config)
	case "azure-monitor":
		return scalers.NewAzureMonitorScaler(config)
	case "azure-pipelines":
		return scalers.NewAzurePipelinesScaler(ctx, config)
	case "azure-queue":
		return scalers.NewAzureQueueScaler(config)
	case "azure-servicebus":
		return scalers.NewAzureServiceBusScaler(ctx, config)
	case "beanstalkd":
		return scalers.NewBeanstalkdScaler(config)
	case "cassandra":
		return scalers.NewCassandraScaler(config)
	case "couchdb":
		return scalers.NewCouchDBScaler(ctx, config)
	case "cron":
		return scalers.NewCronScaler(config)
	case "datadog":
		return scalers.NewDatadogScaler(ctx, config)
	case "dynatrace":
		return scalers.NewDynatraceScaler(config)
	case "elasticsearch":
		return scalers.NewElasticsearchScaler(config)
	case "etcd":
		return scalers.NewEtcdScaler(config)
	case "external":
		return scalers.NewExternalScaler(config)
	// TODO: use other way for test.
	case "external-mock":
		return scalers.NewExternalMockScaler(config)
	case "external-push":
		return scalers.NewExternalPushScaler(config)
	case "gcp-cloudtasks":
		return scalers.NewGcpCloudTasksScaler(config)
	case "gcp-pubsub":
		return scalers.NewPubSubScaler(config)
	case "gcp-stackdriver":
		return scalers.NewStackdriverScaler(ctx, config)
	case "gcp-storage":
		return scalers.NewGcsScaler(config)
	case "github-runner":
		return scalers.NewGitHubRunnerScaler(config)
	case "graphite":
		return scalers.NewGraphiteScaler(config)
	case "huawei-cloudeye":
		return scalers.NewHuaweiCloudeyeScaler(config)
	case "ibmmq":
		return scalers.NewIBMMQScaler(config)
	case "influxdb":
		return scalers.NewInfluxDBScaler(config)
	case "kafka":
		return scalers.NewKafkaScaler(ctx, config)
	case "liiklus":
		return scalers.NewLiiklusScaler(config)
	case "loki":
		return scalers.NewLokiScaler(config)
	case "metrics-api":
		return scalers.NewMetricsAPIScaler(config)
	case "mongodb":
		return scalers.NewMongoDBScaler(ctx, config)
	case "mssql":
		return scalers.NewMSSQLScaler(config)
	case "mysql":
		return scalers.NewMySQLScaler(config)
	case "nats-jetstream":
		return scalers.NewNATSJetStreamScaler(config)
	case "new-relic":
		return scalers.NewNewRelicScaler(config)
	case "nsq":
		return scalers.NewNSQScaler(config)
	case "openstack-metric":
		return scalers.NewOpenstackMetricScaler(ctx, config)
	case "openstack-swift":
		return scalers.NewOpenstackSwiftScaler(config)
	case "postgresql":
		return scalers.NewPostgreSQLScaler(ctx, config)
	case "predictkube":
		return scalers.NewPredictKubeScaler(ctx, config)
	case "prometheus":
		return scalers.NewPrometheusScaler(config)
	case "pulsar":
		return scalers.NewPulsarScaler(config)
	case "rabbitmq":
		return scalers.NewRabbitMQScaler(config)
	case "redis":
		return scalers.NewRedisScaler(ctx /*isClustered*/, false /*isSentinel*/, false, config)
	case "redis-cluster":
		return scalers.NewRedisScaler(ctx /*isClustered*/, true /*isSentinel*/, false, config)
	case "redis-cluster-streams":
		return scalers.NewRedisStreamsScaler(ctx /*isClustered*/, true /*isSentinel*/, false, config)
	case "redis-sentinel":
		return scalers.NewRedisScaler(ctx /*isClustered*/, false /*isSentinel*/, true, config)
	case "redis-sentinel-streams":
		return scalers.NewRedisStreamsScaler(ctx /*isClustered*/, false /*isSentinel*/, true, config)
	case "redis-streams":
		return scalers.NewRedisStreamsScaler(ctx /*isClustered*/, false /*isSentinel*/, false, config)
	case "selenium-grid":
		return scalers.NewSeleniumGridScaler(config)
	case "solace-event-queue":
		return scalers.NewSolaceScaler(config)
	case "solr":
		return scalers.NewSolrScaler(config)
	case "splunk":
		return scalers.NewSplunkScaler(config)
	case "stan":
		return scalers.NewStanScaler(config)
	case "temporal":
		return scalers.NewTemporalScaler(ctx, config)
	default:
		return nil, fmt.Errorf("no scaler found for type: %s", triggerType)
	}
}
