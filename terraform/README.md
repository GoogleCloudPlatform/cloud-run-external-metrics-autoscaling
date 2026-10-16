# Terraform module for CREMA

This directory contains a Terraform module to deploy the Cloud Run External Metrics Autoscaling (CREMA) service.

## Usage

You can use this module in your Terraform code to easily deploy and configure the CREMA service. Before deploying the module, ensure you have created a Service Account for CREMA and a Parameter Manager Parameter Version that contains the CREMA configuration.

### Example

```hcl
# Create the Service Account for CREMA
resource "google_service_account" "crema_sa" {
  account_id   = "crema-service-account"
  display_name = "CREMA Service Account"
}

# Grant necessary permissions to the CREMA Service Account
resource "google_project_iam_member" "crema_sa_permissions" {
  for_each = toset([
    "roles/parametermanager.parameterViewer",
    "roles/run.developer",
    "roles/iam.serviceAccountUser"
  ])

  project = data.google_project.project.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.crema_sa.email}"
}

# Create Parameter Manager parameter for CREMA configuration
resource "google_parameter_manager_parameter" "crema_config" {
  parameter_id = "crema-config"
  format       = "YAML"
}

# Add your CREMA configuration as a version
resource "google_parameter_manager_parameter_version" "crema_config_version" {
  parameter            = google_parameter_manager_parameter.crema_config.id
  parameter_version_id = "1"
  parameter_data       = file("my-crema-config.yaml")
}

# Deploy CREMA using this module
module "crema" {
  source = "github.com/GoogleCloudPlatform/cloud-run-external-metrics-autoscaling//terraform"

  project_id            = var.project_id
  region                = var.region
  service_account_email = google_service_account.crema_sa.email
  crema_config          = google_parameter_manager_parameter_version.crema_config_version.id

  # Optional settings
  # service_name          = "crema"
  # output_scaler_metrics = false
  # enable_cloud_logging  = false
  # log_format            = "json"
}
```

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| `project_id` | The project ID to deploy CREMA to. | `string` | n/a | yes |
| `region` | The region to deploy the CREMA Cloud Run service to. | `string` | n/a | yes |
| `service_account_email` | The email address of the service account used by the CREMA service. | `string` | n/a | yes |
| `crema_config` | The fully qualified name (FQN) of the parameter version in Parameter Manager which contains the CREMA config. | `string` | n/a | yes |
| `service_name` | The name of the CREMA Cloud Run service. | `string` | `"crema"` | no |
| `image` | The CREMA container image to deploy. | `string` | `"us-central1-docker.pkg.dev/cloud-run-oss-images/crema-v1/autoscaler:1.3"` | no |
| `base_image` | The base image to use for automatic base image updates. | `string` | `"us-central1-docker.pkg.dev/serverless-runtimes/google-24/runtimes/java25"` | no |
| `output_scaler_metrics` | Whether CREMA should emit metrics to Cloud Monitoring. | `bool` | `false` | no |
| `enable_cloud_logging` | Whether CREMA should log errors to Cloud Logging for improved log searchability. | `bool` | `false` | no |
| `log_format` | If set to `json`, CREMA will output JSON-structured payloads natively instead of legacy plain-text logs. | `string` | `""` | no |

## Outputs

| Name | Description |
|------|-------------|
| `service_id` | An identifier for the CREMA Cloud Run service with format `projects/{{project}}/locations/{{location}}/services/{{name}}`. |
| `service_uri` | The main URI in which the CREMA Cloud Run service is serving traffic. |
| `service_name` | The name of the CREMA Cloud Run service. |
