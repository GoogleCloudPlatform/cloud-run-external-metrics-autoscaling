# Configuration

CREMA is configured via `CremaConfig`, which consists of KEDA `ScaledObject` and `TriggerAuthentication` objects.

Collectively, these objects allow you to specify:
- Which Cloud Run services or worker pools to scale (`scaleTargetRef`)
- The external metrics to scale on (`triggers`)
- The credentials for accessing those external metrics (`authenticationRef`)
- Autoscaling policy (`horizontalPodAutoscalerConfig`)

An example `CremaConfig`:

```yaml
apiVersion: crema/v1
kind: CremaConfig
spec:
  pollingInterval: 30 # Optional. Seconds
  triggerAuthentications:
    - metadata: # See https://keda.sh/docs/2.17/concepts/authentication
        name: $TRIGGER_AUTH_NAME
      spec:
        gcpSecretManager:
          secrets:
            - parameter: personalAccessToken
              id: $SECRET_NAME
              version: latest
  scaledObjects:
    - spec:
        scaleTargetRef:
          name: projects/$PROJECT_ID/locations/$SERVICE_REGION/workerpools/$WORKER_POOL_NAME
        minReplicaCount: 0 # Optional. Default: 0
        maxReplicaCount: 100 # Optional. Default: 100
        triggers: # See https://keda.sh/docs/2.17/scalers/
          - type: github-runner
            name: my-github-runner-trigger
            metadata:
              owner: acme
              runnerScope: repo
              repos: my-repo
              targetWorkflowQueueLength: 1
            authenticationRef:
              name: $TRIGGER_AUTH_NAME
        advanced: # Optional
          horizontalPodAutoscalerConfig:
            behavior:
              scaleDown:
                stabilizationWindowSeconds: 10
                policies:
                  - type: Pods
                    value: 5
                    periodSeconds: 10
              scaleUp:
                stabilizationWindowSeconds: 10
                policies:
                  - type: Percent
                    value: 100
                    periodSeconds: 10
```
The example configuration scales a Cloud Run worker pool (`$WORKER_POOL_NAME`) using `github-runner` metrics. It uses a GCP Secret Manager secret (`$SECRET_NAME`) to authenticate with Github for reading metrics.

## Invocation

`pollingInterval` is an optional field that controls the interval (in seconds) at which CREMA refreshes its metrics. If specified, CREMA will invoke itself in a loop at the specified interval. As part of each invocation, it will retrieve the specified metrics and scale your Cloud Run workload. 

If `pollingInterval` is not specified, the CREMA service will only be invoked via `POST` to your service's Cloud Run URL.

## TriggerAuthentication

If your external metric source requires credentials, you can provide authentication information via `TriggerAuthentication` objects  ([reference](https://keda.sh/docs/2.17/concepts/authentication/#re-use-credentials-and-delegate-auth-with-triggerauthentication)). You can provide a list of `TriggerAuthentication` objects under the `triggerAuthentications` field.

CREMA supports the following authentication methods:
- [GCP Pod Identity](https://keda.sh/docs/2.17/concepts/authentication/#gcp-pod-identity): Use the CREMA service account's identity ([Application Default Credentials](https://docs.cloud.google.com/docs/authentication/application-default-credentials)) to authenticate. **This is the recommended authentication method for GCP metric sources.**
- [GCP Secret Manager Secrets](https://keda.sh/docs/2.17/concepts/authentication/#gcp-secret-manager-secrets): All secrets must be in the same GCP project as the CREMA instance. The provided `id` should be your secret's name, not its fully qualified name.

### GCP Pod Identity

You can specify the `gcp` Pod Identity provider to authenticate with GCP services (e.g. Pub/Sub, Stackdriver) using the CREMA service account's identity.

```yaml
spec:
  triggerAuthentications:
    - metadata:
        name: $TRIGGER_AUTH_NAME
      spec:
        podIdentity:
          provider: gcp
```

### GCP Secret Manager

You'll have to grant the CREMA service account permission to access those secrets e.g. `Secret Manager Secret Accessor` role (`roles/secretmanager.secretAccessor`) on any secrets referenced in your configuration.

```bash
SECRET_NAME=my-secret

gcloud secrets add-iam-policy-binding $SECRET_NAME \
    --member="serviceAccount:$SERVICE_ACCOUNT_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"
```

## ScaledObject

Each `ScaledObject` specifies a Cloud Run service or worker pool to scale, the metric(s) to scale it on, and an optional autoscaling policy.

The following parts are defined for each scaled object.

### scaleTargetRef

The `scaleTargetRef` is required. It specifies the Cloud Run service or worker pool to scale. The name must be the workload's FQN, e.g.
```bash
projects/$MY_PROJECT/locations/$REGION/services/$SERVICE_NAME
projects/$MY_PROJECT/locations/$REGION/workerpools/$WORKER_POOL_NAME
```

### min/maxReplicaCount

You can optionally set `minReplicaCount` and/or `maxReplicaCount` to bound scaling thereby controlling your costs. 

### triggers

Multiple external metric sources can be used to scale. Each entry in the `triggers` list corresponds to an external metric to scale on. Please see the [KEDA reference](https://keda.sh/docs/2.17/scalers/) for configuration details for each metric source type.

If your external metric source requires authentication, set the `authenticationRef` field to specify one of the `TriggerAuthentication` objects to use for credentials.

### horizontalPodAutoscalerConfig

You can optionally configure scaling behavior via `horizontalPodAutoscalerConfig`. Please see the [Kubernetes documentation](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#configurable-scaling-behavior) for details.

If a `horizontalPodAutoscalerConfig` is not specified, CREMA will apply HPA's [default behavior](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#default-behavior).

If only `scaleDown` is specified, CREMA will apply `scaleUp` from HPA's default behavior.

If only `scaleUp` is specified, CREMA will apply `scaleDown` from HPA's default behavior.

# Appendix: Migrating from KEDA

Because `CremaConfig` is composed of KEDA configuration objects, you can mostly copy and paste your existing KEDA `TriggerAuthentication` and `ScaledObject` configurations into the `triggerAuthentications` and `scaledObjects` lists. There are some things to keep in mind though:
- CREMA only supports `gcpSecretsManager` and `podIdentity` (with `provider: gcp`) from KEDA's `TriggerAuthentication`; all other authentication provider fields are ignored.
- The `scaleTargetRef` must refer to a Cloud Run service or worker pool.

The following KEDA `ScaledObject` fields are not supported and will be ignored by CREMA:
- `envSourceContainerName`
- `cooldownPeriod`
- `initialCooldownPeriod`
- `fallback`
- `restoreToOriginalReplicaCount`
- `pollingInterval`: the polling interval is defined for the entire CREMA service rather than per `ScaledObject`; if you want different intervals per `ScaledObject`, you will have to create separate CREMA services.

# Appendix: Setting up Cloud Scheduler to invoke CREMA
If you don't need to refresh your metrics more frequently than once every minute, you can create a Cloud Scheduler job to periodically invoke CREMA to take advantage of Cloud Run's request-based billing to minimize costs.

Follow the steps below to set this up:

1. Create a service account for the Cloud Scheduler job to use:
   ```bash
   SCHEDULER_SA_NAME=scheduler-invoker

   gcloud iam service-accounts create $SCHEDULER_SA_NAME \
     --display-name "Cloud Scheduler Invoker"
   ```

2. Grant the service account permission to invoke your CREMA service:
   ```bash
   CREMA_SERVICE_NAME=my-crema-service

   gcloud run services add-iam-policy-binding $CREMA_SERVICE_NAME \
     --region=$SERVICE_REGION \
     --member="serviceAccount:$SCHEDULER_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com" \
     --role="roles/run.invoker"
   ```

3. Create a Cloud Scheduler job to invoke your CREMA service endpoint. This example sets up a job that runs every minute.
   ```bash
   SCHEDULER_JOB_REGION=us-central1
   CREMA_SERVICE_URL=https://crema-426654618204.us-central1.run.app

   gcloud scheduler jobs create http crema-scheduler \
     --location=$SCHEDULER_JOB_REGION \
     --schedule="* * * * *" \
     --uri="$CREMA_SERVICE_URL" \
     --http-method=POST \
     --oidc-service-account-email="$SCHEDULER_SA_NAME@$PROJECT_ID.iam.gserviceaccount.com"
   ```