# Cloud Run External Metrics Autoscaling

The Cloud Run External Metrics Autoscaling (CREMA) project leverages [KEDA](https://github.com/kedacore/keda) to provide autoscaling for Cloud Run services and worker pools.

# Compatibility
This project currently depends on **KEDA v2.17**. The included table lists various KEDA scalers and their compatibility for use with Cloud Run.

| Scalers                                                                                   | Cloud Run Compatible |
|:------------------------------------------------------------------------------------------|:---------------------|
| [Apache Kafka](https://keda.sh/docs/2.17/scalers/apache-kafka/)                           | Verified             |
| [CPU](https://keda.sh/docs/2.17/scalers/cpu/)                                             | Incompatible         |
| [Cron](https://keda.sh/docs/2.17/scalers/cron/)                                           | Verified             |
| [Github Runner Scaler](https://keda.sh/docs/2.17/scalers/github-runner/)                  | Verified             |
| [Kubernetes Workload](https://keda.sh/docs/2.17/scalers/kubernetes-workload/)             | Incompatible         |
| [Memory](https://keda.sh/docs/2.17/scalers/memory/)                                       | Incompatible         |
| [Redis Lists](https://keda.sh/docs/2.17/scalers/redis-lists/)                             | Verified             |

See https://keda.sh/docs/2.17/scalers/ for the full list of KEDA's scalers. The compatibility for any KEDA scaler not listed above is currently unknown. Please file an issue if you believe a scaler does not work.

# Setup

Follow the instructions below to build, configure, deploy, and verify your CREMA service.

## Prerequisites

1.  **Google Cloud SDK:** Ensure you have the [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) installed and configured.
2.  **Authentication:** Authenticate with Google Cloud:
    ```bash
    gcloud auth login
    gcloud auth application-default login
    ```
3.  **Project Configuration:** Set your default project:
    ```bash
    gcloud config set project MY_PROJECT_ID
    ```
    Replace `MY_PROJECT_ID` with your actual Google Cloud project ID.

## Create a GCP Service Account

Create a GCP service account that will be used by the Cloud Run CREMA service. We'll grant this service account the necessary permissions throughout the setup. Those permissions will be:
- `Parameter Manager Parameter Viewer` (`roles/parametermanager.parameterViewer`) to retrieve from Parameter Manager the CREMA configuration you'll be creating.
- `Artifact Registry Reader` (`roles/artifactregistry.reader`) to access the container images.
- `Cloud Run Developer` (`roles/run.developer`) to set the number of instances in your scaled workloads.

```bash
PROJECT_ID=my-project
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud iam service-accounts create $CREMA_SERVICE_ACCOUNT_NAME \
  --display-name="CREMA Service Account"
```

## Build

Follow the steps below to build the CREMA service and make the resulting container image available in Artifact Registry.

Create an Artifact Registry repository to store the CREMA container image if you don't already have one:

```bash
PROJECT_ID=my-project
CREMA_REPO_NAME=crema
AR_REGION=us-central1

gcloud artifacts repositories create "${CREMA_REPO_NAME}" --repository-format=docker --location=$AR_REGION --description="Docker repository for CREMA images"
```

Use Google Cloud Build and the included `Dockerfile` to build the container image and push it to Artifact Registry. Run the following command from the root of this project:

```bash
gcloud builds submit --tag $AR_REGION-docker.pkg.dev/$PROJECT_ID/$CREMA_REPO_NAME/crema:latest .
```

Note that this build process may take 30+ minutes.

## Configure

Follow the steps below to create a yaml configuration file for CREMA in [Parameter Manager](https://docs.cloud.google.com/secret-manager/parameter-manager).

Create a Parameter in Parameter Manager to store your CREMA config. This parameter is where you will store Parameter Versions to be used by CREMA:
```bash
PARAMETER_ID=crema-config
PARAMETER_REGION=global
gcloud parametermanager parameters create $PARAMETER_ID --location=$PARAMETER_REGION --parameter-format=YAML
```

Locally, create a YAML file for your CREMA configuration. See the [Configuration README](metric-provider/api/README.md) for reference.

Upload your local YAML file as a new parameter version:

```bash
LOCAL_YAML_CONFIG_FILE=./my-crema-config.yaml
PARAMETER_VERSION=1

gcloud parametermanager parameters versions create $PARAMETER_VERSION \
  --location=$PARAMETER_REGION \
  --parameter=$PARAMETER_ID \
  --payload-data-from-file=$LOCAL_YAML_CONFIG_FILE
```

Grant your CREMA service account permission to read from Parameter Manager:

```bash
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/parametermanager.parameterViewer"
```

Grant your CREMA service account permission to scale the services and worker pools specified in your configuration.

Both of the following steps must be completed **for each service and worker pool that your CREMA service scales**.

1. Grant the CREMA service account the `Cloud Run Developer` role on the service / worker pool:
```bash
SERVICE_NAME=my-service-to-be-scaled
SERVICE_REGION=us-central1
gcloud run services add-iam-policy-binding $SERVICE_NAME \
  --region=$SERVICE_REGION \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.developer"
```

*or*

```bash
WORKER_POOL_NAME=my-worker-pool-to-be-scaled
WORKER_POOL_REGION=us-central1
gcloud alpha run worker-pools add-iam-policy-binding $WORKER_POOL_NAME \
  --region=$WORKER_POOL_REGION \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.developer"
```

2. Grant the CREMA service account the `Artifact Registry Reader` role on the artifacts that are deployed by the service / worker pool:
```bash
SERVICE_AR_REPO=my-service-to-be-scaled-repo
gcloud artifacts repositories add-iam-policy-binding $SERVICE_AR_REPO \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/artifactregistry.reader"
```

## Deploy

Deploy a CREMA service using the built image with the following `gcloud` command.

Configure and run the following command:
- `SERVICE_NAME`: The name for your CREMA service
- `SERVICE_REGION`: The region to run your CREMA service in.
- `CREMA_SERVICE_ACCOUNT_NAME`: The name of the service account which will run CREMA
- `PARAMETER_VERSION`: The parameter version you created

```bash
SERVICE_NAME=my-crema-service
SERVICE_REGION=us-central1
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

CREMA_CONFIG_PARAM_VERSION=projects/$PROJECT_ID/locations/$PARAMETER_REGION/parameters/$PARAMETER_ID/versions/$PARAMETER_VERSION
IMAGE=$AR_REGION-docker.pkg.dev/$PROJECT_ID/$CREMA_REPO_NAME/crema:latest

gcloud beta run deploy $SERVICE_NAME \
  --image=${IMAGE} \
  --region=${SERVICE_REGION} \
  --service-account="${CREMA_SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
  --no-allow-unauthenticated \
  --no-cpu-throttling \
  --base-image=us-central1-docker.pkg.dev/serverless-runtimes/google-22/runtimes/java21 \
  --labels=created-by=crema \
  --set-env-vars="CREMA_CONFIG=${CREMA_CONFIG_PARAM_VERSION},OUTPUT_SCALER_METRICS=False,ENABLE_CLOUD_LOGGING=False"
```

The following environment variables are checked by the container:
- `CREMA_CONFIG`: Required. The fully qualified name (FQN) of the parameter version which contains your CREMA config.
- `OUTPUT_SCALER_METRICS`: Optional. If true, CREMA will emit metrics to Cloud Monitoring.
- `ENABLE_CLOUD_LOGGING`: Optional. If true, CREMA will log errors to Cloud Logging for improved log searchability.

Note: The `OUTPUT_SCALER_METRICS` and `ENABLE_CLOUD_LOGGING` flags are disabled by default as these may incur additional costs. See [Cloud Observability Pricing](https://cloud.google.com/products/observability/pricing) for details.

If you set the `OUTPUT_SCALER_METRICS=True` environment variable, you'll also have to grant your CREMA service account permission to write metrics:

```bash
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/monitoring.metricWriter"
```

If you set the `ENABLE_CLOUD_LOGGING=True` environment variable, you'll also have to grant your CREMA service account permission to write log entries:

```bash
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/logging.logWriter"
```

## Verify

Use the resource below to verify that your CREMA service is running correctly.

### Service Logs
When your CREMA service is running, you should see the following logs in your service's logs each time metrics are refreshed:

Each log message is labeled with the component that emitted it.
```
[INFO] [METRIC-PROVIDER] Starting metric collection cycle
[INFO] [METRIC-PROVIDER] Successfully fetched scaled object metrics ...
[INFO] [METRIC-PROVIDER] Sending scale request ...
[INFO] [SCALER] Received ScaleRequest ...
[INFO] [SCALER] Current instances ...
[INFO] [SCALER] Recommended instances ...
```

TIP: Use the following Cloud Logging query for filtering the CREMA service's logs: `"[SCALER]" OR "[METRIC-PROVIDER]"`

### Metrics
If configured, CREMA will emit the following metrics:
- `custom.googleapis.com/$TRIGGER_TYPE/metric_value`: The metric value it received, per trigger type
- `custom.googleapis.com/recommended_instance_count`: The number of instances recommended, per Cloud Run scaled object
- `custom.googleapis.com/requested_instance_count`: The number of instances requested, per Cloud Run scaled object
