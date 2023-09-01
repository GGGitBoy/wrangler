/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by main. DO NOT EDIT.

package v1

import (
	"context"
	"time"

	"github.com/rancher/wrangler/v2/pkg/apply"
	"github.com/rancher/wrangler/v2/pkg/condition"
	"github.com/rancher/wrangler/v2/pkg/generic"
	"github.com/rancher/wrangler/v2/pkg/kv"
	v1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// JobController interface for managing Job resources.
type JobController interface {
	generic.ControllerInterface[*v1.Job, *v1.JobList]
}

// JobClient interface for managing Job resources in Kubernetes.
type JobClient interface {
	generic.ClientInterface[*v1.Job, *v1.JobList]
}

// JobCache interface for retrieving Job resources in memory.
type JobCache interface {
	generic.CacheInterface[*v1.Job]
}

type JobStatusHandler func(obj *v1.Job, status v1.JobStatus) (v1.JobStatus, error)

type JobGeneratingHandler func(obj *v1.Job, status v1.JobStatus) ([]runtime.Object, v1.JobStatus, error)

func RegisterJobStatusHandler(ctx context.Context, controller JobController, condition condition.Cond, name string, handler JobStatusHandler) {
	statusHandler := &jobStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, generic.FromObjectHandlerToHandler(statusHandler.sync))
}

func RegisterJobGeneratingHandler(ctx context.Context, controller JobController, apply apply.Apply,
	condition condition.Cond, name string, handler JobGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &jobGeneratingHandler{
		JobGeneratingHandler: handler,
		apply:                apply,
		name:                 name,
		gvk:                  controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterJobStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type jobStatusHandler struct {
	client    JobClient
	condition condition.Cond
	handler   JobStatusHandler
}

func (a *jobStatusHandler) sync(key string, obj *v1.Job) (*v1.Job, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type jobGeneratingHandler struct {
	JobGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *jobGeneratingHandler) Remove(key string, obj *v1.Job) (*v1.Job, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1.Job{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *jobGeneratingHandler) Handle(obj *v1.Job, status v1.JobStatus) (v1.JobStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.JobGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
