
variable "project_id" {
  type        = string
  description = "The project ID to deploy CREMA to."
}

variable "region" {
  type        = string
  description = "The region to deploy the CREMA Cloud Run service to."
}

variable "service_name" {
  type        = string
  description = "The name of the CREMA Cloud Run service."
  default     = "crema"
}

variable "service_account_email" {
  type        = string
  description = "The email address of the service account used by the CREMA service."
}

variable "crema_config" {
  type        = string
  description = "The fully qualified name (FQN) of the parameter version in Parameter Manager which contains the CREMA config. Example: projects/MY_PROJECT/locations/global/parameters/crema-config/versions/1"
}

variable "image" {
  type        = string
  description = "The CREMA container image to deploy."
  default     = "us-central1-docker.pkg.dev/cloud-run-oss-images/crema-v1/autoscaler:1.3"
}

variable "base_image" {
  type        = string
  description = "The base image to use for automatic base image updates."
  default     = "us-central1-docker.pkg.dev/serverless-runtimes/google-24/runtimes/java25"
}

variable "output_scaler_metrics" {
  type        = bool
  description = "Whether CREMA should emit metrics to Cloud Monitoring."
  default     = false
}

variable "enable_cloud_logging" {
  type        = bool
  description = "Whether CREMA should log errors to Cloud Logging for improved log searchability."
  default     = false
}

variable "deletion_protection" {
  type        = bool
  description = "Whether Terraform will be prevented from destroying or recreating the service."
  default     = true
}

variable "log_format" {
  type        = string
  description = "If set to 'json', CREMA will output JSON-structured payloads natively instead of legacy plain-text logs."
  default     = ""
}
