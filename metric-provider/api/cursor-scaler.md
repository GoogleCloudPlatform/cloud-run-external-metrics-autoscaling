# Native Cursor Scaler for Cloud Run CREMA

This document explains how the native `cursor` scaler is designed, how it works, and how to configure and use it within your CREMA deployment.

---

## How It Works

The native `cursor` scaler performs the following operations:
1. **API Polling**: Periodically queries the Cursor private workers API endpoint `https://api.cursor.com/v0/private-workers/summary` using the configured `CURSOR_API_KEY`.
2. **Metrics Extraction**: Retrieves the number of private workers currently `inUse` (for either Team or User scopes).
3. **HPA Reconcilation**: Returns the `inUse` metric value to CREMA's scaler service.
4. **Dynamic Scaling**: The CREMA Java scaling controller computes the desired number of instances using **Target Average Value Scaling**:
   $$\text{Desired Instances} = \lceil \frac{\text{Workers In Use}}{\text{Target Average Value}} \rceil$$
   *For example, with $2$ workers in use and a target average value (utilization) of $0.5$, CREMA automatically scales the Worker Pool to $\lceil \frac{2}{0.5} \rceil = 4$ instances.*

---

## Configuration Guide

You can define `cursor` triggers directly in your `CremaConfig` YAML parameters:

### 1. Set up the Secret Manager Authentication
First, create a Secret in Google Cloud Secret Manager (e.g. named `CURSOR_API_KEY`) and store your team-level or user-level Cursor API key there.

Then, map this secret in your `CremaConfig`'s `triggerAuthentications`:

```yaml
apiVersion: crema/v1
kind: CremaConfig
spec:
  triggerAuthentications:
    - metadata:
        name: cursor-auth
      spec:
        gcpSecretManager:
          secrets:
            - parameter: apiKey
              id: CURSOR_API_KEY
              version: latest
```

### 2. Configure the ScaledObject Trigger
Next, define a `scaledObjects` entry referencing your Cloud Run Worker Pool and configure the `cursor` trigger:

```yaml
  scaledObjects:
    - spec:
        scaleTargetRef:
          # The FQN of your Cursor worker pool
          name: projects/YOUR_PROJECT_ID/locations/us-central1/workerpools/cursor-agents-pool
        
        minReplicaCount: 1  # Minimum pool size
        maxReplicaCount: 50 # Maximum pool size
        
        triggers:
          - type: cursor
            name: cursor-utilization-trigger
            metricType: AverageValue
            metadata:
              # Target utilization value (e.g. 0.5 means scale up aggressively when 50% in use)
              value: "0.5"
            authenticationRef:
              name: cursor-auth
        
        advanced:
          horizontalPodAutoscalerConfig:
            behavior:
              scaleDown:
                stabilizationWindowSeconds: 120 # Gradual scaling down to prevent churn
              scaleUp:
                stabilizationWindowSeconds: 0   # Instant scale up for responsive execution
```
