> [!NOTE]
> This feature is available at the [Preview](https://cloud.google.com/products?e=48754805#product-launch-stages) release level.

# Cloud Run External Metrics Autoscaling

The Cloud Run External Metrics Autoscaling (CREMA) project leverages [KEDA](https://github.com/kedacore/keda) to provide autoscaling for Cloud Run services and worker pools.

# Compatibility
This project currently depends on **KEDA v2.17**. The included table lists various KEDA scalers and their compatibility for use with Cloud Run.

| Scalers                                                                                   | Cloud Run Compatible |
|:------------------------------------------------------------------------------------------|:---------------------|
| [Apache Kafka](https://keda.sh/docs/2.17/scalers/apache-kafka/)                           | Verified             |
| [Cron](https://keda.sh/docs/2.17/scalers/cron/)                                           | Verified             |
| [Github Runner Scaler](https://keda.sh/docs/2.17/scalers/github-runner/)                  | Verified             |
| [CPU](https://keda.sh/docs/2.17/scalers/cpu/)                                             | Incompatible         |
| [Kubernetes Workload](https://keda.sh/docs/2.17/scalers/kubernetes-workload/)             | Incompatible         |
| [Memory](https://keda.sh/docs/2.17/scalers/memory/)                                       | Incompatible         |

See https://keda.sh/docs/2.17/scalers/ for the full list of KEDA's scalers. The compatibility for any KEDA scaler not listed above is currently unknown. Please file an issue if you believe a scaler does not work.

# Setup

Follow the instructions below to configure and deploy CREMA as a Cloud Run worker pool to scale your Cloud Run workloads on metrics external to Cloud Run.

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

Create a GCP service account that will be used by the Cloud Run CREMA worker pool. We'll grant this service account the necessary permissions throughout the setup. Those permissions will be:
- `Parameter Manager Parameter Viewer` (`roles/parametermanager.parameterViewer`) to retrieve from Parameter Manager the CREMA configuration you'll be creating.
- `Cloud Run Developer` (`roles/run.developer`) and `Service Account User` (`roles/iam.serviceAccountUser`) to set the number of instances in your scaled workloads.

```bash
PROJECT_ID=my-project
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud iam service-accounts create $CREMA_SERVICE_ACCOUNT_NAME \
  --display-name="CREMA Service Account"
```

## Configure

Follow the steps below to create a yaml configuration file for CREMA in [Parameter Manager](https://docs.cloud.google.com/secret-manager/parameter-manager).

Create a Parameter in Parameter Manager to store the configuration used by CREMA:
```bash
PARAMETER_ID=crema-config
PARAMETER_REGION=global
gcloud parametermanager parameters create $PARAMETER_ID --location=$PARAMETER_REGION --parameter-format=YAML
```

Locally, create a YAML file for your CREMA configuration. See the [Configuration README](metric-provider/api/README.md) for reference.

Upload your local YAML file to Parameter Manager as a new parameter version:

```bash
LOCAL_YAML_CONFIG_FILE=./my-crema-config.yaml
PARAMETER_ID=crema-config
PARAMETER_REGION=global
PARAMETER_VERSION=1

gcloud parametermanager parameters versions create $PARAMETER_VERSION \
  --location=$PARAMETER_REGION \
  --parameter=$PARAMETER_ID \
  --payload-data-from-file=$LOCAL_YAML_CONFIG_FILE
```

Grant your CREMA service account permission to read the parameter version from Parameter Manager:

```bash
PROJECT_ID=my-project
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/parametermanager.parameterViewer"
```

Grant your CREMA service account permission to scale the workloads that you've specified in your CREMA configuration. This can be done by granting `roles/run.developer` at the project level or for each individual service or worker pool to be scaled.

Granting the required permissions at the project level will enable CREMA to scale any workloads that you specify in the configuration--you'll be able to add more workloads in the future without having to further modify permissions. To grant these permissions at the project level:

```bash
PROJECT_ID=my-project
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.developer"
```

Alternatively, you can grant the required permissions for each individual service or worker pool. This minimizes the permissions to strictly what's necessary and is considered a security best practice. To grant these permissions for each individual service or worker pool:

```bash
# For a service
PROJECT_ID=my-project
SERVICE_NAME=my-service-to-be-scaled
SERVICE_REGION=us-central1
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account


gcloud run services add-iam-policy-binding $SERVICE_NAME \
  --region=$SERVICE_REGION \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.developer"

# For a worker pool
PROJECT_ID=my-project
WORKER_POOL_NAME=my-worker-pool-to-be-scaled
WORKER_POOL_REGION=us-central1
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud beta run worker-pools add-iam-policy-binding $WORKER_POOL_NAME \
  --region=$WORKER_POOL_REGION \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/run.developer"
```

Grant your CREMA service account `roles/iam.serviceAccountUser` on the service accounts which run the Cloud Run workloads to be scaled:

```bash
PROJECT_ID=my-project
CONSUMER_SERVICE_ACCOUNT_NAME=my-worker-pool-sa
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud iam service-accounts add-iam-policy-binding \
    $CONSUMER_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com \
    --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/iam.serviceAccountUser"
```

## Deploy

Deploy your CREMA worker pool using either
- our pre-built container image in `us-central1-docker.pkg.dev/cloud-run-oss-images/crema-v1/autoscaler`
- or a container image you build yourself from the source code using Cloud Build (see [instructions](#optional-build-the-container-image-from-source) below).

The command here deploys CREMA as a Cloud Run worker pool using the pre-built container image; if you want to deploy your own built container image, update the IMAGE variable to specify it.

Configure the variables and the command deploy command:
- `WORKER_POOL_NAME`: The name for your CREMA worker pool
- `WORKER_POOL_REGION`: The region to run your CREMA worker pool in.
- `CREMA_SERVICE_ACCOUNT_NAME`: The name of the service account which will run CREMA
- `PARAMETER_VERSION`: The parameter version you created

```bash
WORKER_POOL_NAME=my-crema-worker-pool
WORKER_POOL_REGION=us-central1
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account
PARAMETER_VERSION=1

CREMA_CONFIG_PARAM_VERSION=projects/$PROJECT_ID/locations/$PARAMETER_REGION/parameters/$PARAMETER_ID/versions/$PARAMETER_VERSION
IMAGE=us-central1-docker.pkg.dev/cloud-run-oss-images/crema-v1/autoscaler:1.0

gcloud beta run worker-pools deploy $WORKER_POOL_NAME \
  --image=${IMAGE} \
  --region=${WORKER_POOL_REGION} \
  --service-account="${CREMA_SERVICE_ACCOUNT_NAME}@${PROJECT_ID}.iam.gserviceaccount.com" \
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
PROJECT_ID=my-project
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/monitoring.metricWriter"
```

If you set the `ENABLE_CLOUD_LOGGING=True` environment variable, you'll also have to grant your CREMA service account permission to write log entries:

```bash
PROJECT_ID=my-project
CREMA_SERVICE_ACCOUNT_NAME=crema-service-account

gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:$CREMA_SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/logging.logWriter"
```

## Verify

Use the resource below to verify that your CREMA worker pool is running correctly.

### Cloud Logging Logs
CREMA writes logs to Cloud Logging during each scaling cycle. You should see the following entries in your worker pool's logs in Cloud Logging each time metrics are refreshed:

Each log message is labeled with the component that emitted it.
```
[INFO] [METRIC-PROVIDER] Starting metric collection cycle
[INFO] [METRIC-PROVIDER] Successfully fetched scaled object metrics ...
[INFO] [METRIC-PROVIDER] Sending scale request ...
[INFO] [SCALER] Received ScaleRequest ...
[INFO] [SCALER] Current instances ...
[INFO] [SCALER] Recommended instances ...
```

TIP: Use the following Cloud Logging query for filtering the CREMA worker pool's logs: `"[SCALER]" OR "[METRIC-PROVIDER]"`

## Optional: Build the container image from source

Follow the steps below to build CREMA and make the resulting container image available in Artifact Registry.

Create an Artifact Registry repository to store the CREMA container image if you don't already have one:

```bash
PROJECT_ID=my-project
CREMA_REPO_NAME=crema
AR_REGION=us-central1

gcloud artifacts repositories create "${CREMA_REPO_NAME}" --repository-format=docker --location=$AR_REGION --description="Docker repository for CREMA images"
```

Use Google Cloud Build and the included `Dockerfile` to build the container image and push it to Artifact Registry. Run the following command from the root of this project:

```bash
PROJECT_ID=my-project
CREMA_REPO_NAME=crema
AR_REGION=us-central1

gcloud builds submit --tag $AR_REGION-docker.pkg.dev/$PROJECT_ID/$CREMA_REPO_NAME/crema:latest .
```

Note that this build process may take 30+ minutes.

### Metrics
If configured, CREMA will emit the following metrics:
- `custom.googleapis.com/$TRIGGER_TYPE/metric_value`: The metric value it received, per trigger type
- `custom.googleapis.com/recommended_instance_count`: The number of instances recommended, per Cloud Run scaled object
- `custom.googleapis.com/requested_instance_count`: The number of instances requested, per Cloud Run scaled object

## Known Issues
*   **CREMA does not currently resolve environment variables:** As a result, KEDA configuration fields which rely on environment variables, i.e. those with a `FromEnv` suffix such as `usernameFromEnv` `passwordFromEnv` from KEDA's [Redis  scaler](https://keda.sh/docs/2.17/scalers/redis-lists/), are not supported.

*   **Slow metrics in Cloud Monitoring:** Many Google Cloud Monitoring metrics have [2+ minute ingestion delay](https://docs.cloud.google.com/monitoring/api/v3/latency-n-retention#latency) which may affect scaling responsiveness for Google Cloud Platform scalers. See the [Google Cloud metrics list](https://docs.cloud.google.com/monitoring/api/metrics_gcp) for the underlying metrics used by the scaler for latency details.

*   **A given Cloud Run service or worker pool should only be scaled by a single CREMA deployment:** Scaling the same service or worker pool from multiple CREMA deployments can lead to race conditions and unexpected scaling behavior.
