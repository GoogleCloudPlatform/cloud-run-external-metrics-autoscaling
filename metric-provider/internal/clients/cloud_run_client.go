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
	"fmt"

	"google.golang.org/api/option"
	"google.golang.org/api/run/v2"
)

type servicesAPI interface {
	getService(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error)
	updateService(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) error
}

type workerPoolsAPI interface {
	getWorkerPool(ctx context.Context, name string) (*run.GoogleCloudRunV2WorkerPool, error)
	updateWorkerPool(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) error
}

type runServicesAPI struct {
	svc *run.ProjectsLocationsServicesService
}

func (r *runServicesAPI) getService(ctx context.Context, name string) (*run.GoogleCloudRunV2Service, error) {
	return r.svc.Get(name).Context(ctx).Do()
}

func (r *runServicesAPI) updateService(ctx context.Context, name string, service *run.GoogleCloudRunV2Service, updateMask string) error {
	_, err := r.svc.Patch(name, service).UpdateMask(updateMask).Context(ctx).Do()
	return err
}

type runWorkerPoolsAPI struct {
	wp *run.ProjectsLocationsWorkerPoolsService
}

func (r *runWorkerPoolsAPI) getWorkerPool(ctx context.Context, name string) (*run.GoogleCloudRunV2WorkerPool, error) {
	return r.wp.Get(name).Context(ctx).Do()
}

func (r *runWorkerPoolsAPI) updateWorkerPool(ctx context.Context, name string, workerPool *run.GoogleCloudRunV2WorkerPool, updateMask string) error {
	_, err := r.wp.Patch(name, workerPool).UpdateMask(updateMask).Context(ctx).Do()
	return err
}

// CloudRunClient is a thin wrapper around the Cloud Run v2 API.
type CloudRunClient struct {
	services    servicesAPI
	workerPools workerPoolsAPI
}

// NewCloudRunClient creates a new CloudRunClient.
func NewCloudRunClient(ctx context.Context, opts ...option.ClientOption) (*CloudRunClient, error) {
	runService, err := run.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cloud run service client: %w", err)
	}
	return &CloudRunClient{
		services:    &runServicesAPI{svc: run.NewProjectsLocationsServicesService(runService)},
		workerPools: &runWorkerPoolsAPI{wp: run.NewProjectsLocationsWorkerPoolsService(runService)},
	}, nil
}

// GetWorkerPoolInstanceCount returns the instance count for a given worker pool.
// Cloud Run Admin API returns 0/nil if manual instance count is not set; this returns 0 in that case.
func (c *CloudRunClient) GetWorkerPoolInstanceCount(ctx context.Context, workerpoolName, projectID, region string) (int, error) {
	workerPool, err := c.getWorkerPool(ctx, workerpoolName, projectID, region)
	if err != nil {
		return 0, err
	}
	if workerPool.Scaling != nil {
		return int(workerPool.Scaling.ManualInstanceCount), nil
	}
	return 0, nil
}

// UpdateWorkerPoolManualInstances updates the number of manual instances for a given worker pool.
// Waits only until Cloud Run accepts the update; the operation completes asynchronously.
func (c *CloudRunClient) UpdateWorkerPoolManualInstances(ctx context.Context, workerpoolName string, instances int, projectID, region string) error {
	resourceName := workerPoolResourceName(projectID, region, workerpoolName)
	currentWorkerPool, err := c.getWorkerPool(ctx, workerpoolName, projectID, region)
	if err != nil {
		return err
	}

	currentWorkerPool.Scaling = &run.GoogleCloudRunV2WorkerPoolScaling{
		ManualInstanceCount: int64(instances),
		ForceSendFields:     []string{"ManualInstanceCount"},
	}

	return c.workerPools.updateWorkerPool(ctx, resourceName, currentWorkerPool, "scaling.manual_instance_count")
}

// GetServiceInstanceCount returns the instance count for a given service.
// Cloud Run Admin API returns 0/nil if min instances is 0 with autoscaling; this returns 0 in that case.
func (c *CloudRunClient) GetServiceInstanceCount(ctx context.Context, serviceName, projectID, region string) (int, error) {
	service, err := c.getService(ctx, serviceName, projectID, region)
	if err != nil {
		return 0, err
	}
	if service.Scaling != nil && service.Scaling.ScalingMode == "MANUAL" {
		return int(service.Scaling.ManualInstanceCount), nil
	}
	if service.Template != nil && service.Template.Scaling != nil {
		return int(service.Template.Scaling.MinInstanceCount), nil
	}
	return 0, nil
}

// UpdateServiceMinInstances updates the number of min instances for a given service.
// Waits only until Cloud Run accepts the update; the operation completes asynchronously.
func (c *CloudRunClient) UpdateServiceMinInstances(ctx context.Context, serviceName string, instances int, projectID, region string) error {
	resourceName := serviceResourceName(projectID, region, serviceName)
	currentService, err := c.getService(ctx, serviceName, projectID, region)
	if err != nil {
		return err
	}

	var newScaling run.GoogleCloudRunV2RevisionScaling
	if currentService.Template != nil && currentService.Template.Scaling != nil {
		newScaling = *currentService.Template.Scaling
	}
	newScaling.MinInstanceCount = int64(instances)
	newScaling.ForceSendFields = []string{"MinInstanceCount"}

	var newTemplate run.GoogleCloudRunV2RevisionTemplate
	if currentService.Template != nil {
		newTemplate = *currentService.Template
	}
	newTemplate.Scaling = &newScaling
	currentService.Template = &newTemplate

	return c.services.updateService(ctx, resourceName, currentService, "template.scaling.min_instance_count")
}

// UpdateServiceManualInstances updates the number of manual instances for a given service.
// Waits only until Cloud Run accepts the update; the operation completes asynchronously.
func (c *CloudRunClient) UpdateServiceManualInstances(ctx context.Context, serviceName string, instances int, projectID, region string) error {
	resourceName := serviceResourceName(projectID, region, serviceName)
	currentService, err := c.getService(ctx, serviceName, projectID, region)
	if err != nil {
		return err
	}

	currentService.Scaling = &run.GoogleCloudRunV2ServiceScaling{
		ManualInstanceCount: int64(instances),
		ForceSendFields:     []string{"ManualInstanceCount"},
	}

	return c.services.updateService(ctx, resourceName, currentService, "scaling.manual_instance_count")
}

func (c *CloudRunClient) getWorkerPool(ctx context.Context, workerpoolName, projectID, region string) (*run.GoogleCloudRunV2WorkerPool, error) {
	return c.workerPools.getWorkerPool(ctx, workerPoolResourceName(projectID, region, workerpoolName))
}

func (c *CloudRunClient) getService(ctx context.Context, serviceName, projectID, region string) (*run.GoogleCloudRunV2Service, error) {
	return c.services.getService(ctx, serviceResourceName(projectID, region, serviceName))
}

func workerPoolResourceName(projectID, region, workerpoolName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/workerPools/%s", projectID, region, workerpoolName)
}

func serviceResourceName(projectID, region, serviceName string) string {
	return fmt.Sprintf("projects/%s/locations/%s/services/%s", projectID, region, serviceName)
}
