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

package queue

import (
	"context"
	"time"

	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/source"

	kueue "sigs.k8s.io/kueue/apis/kueue/v1beta2"
	qcache "sigs.k8s.io/kueue/pkg/cache/queue"
)

const (
	batchPeriod = 5 * time.Second
)

type requeueRequest struct {
	ClusterQueue kueue.ClusterQueueReference
	Cohort       kueue.CohortReference
}

type InadmissibleWorkloadRequeuer struct {
	client   client.Client
	qManager *qcache.Manager
	eventCh  chan event.TypedGenericEvent[requeueRequest]
}

func NewInadmissibleWorkloadReconciler(client client.Client, qManager *qcache.Manager) *InadmissibleWorkloadRequeuer {
	return &InadmissibleWorkloadRequeuer{
		client:   client,
		qManager: qManager,
		eventCh:  make(chan event.TypedGenericEvent[requeueRequest]),
	}
}

func (r *InadmissibleWorkloadRequeuer) Reconcile(ctx context.Context, req requeueRequest) (ctrl.Result, error) {
	if req.ClusterQueue != "" {
		r.qManager.RequeueClusterQueue(ctx, req.ClusterQueue)
	}
	if req.Cohort != "" {
		r.qManager.RequeueCohort(ctx, req.Cohort)
	}
	return ctrl.Result{}, nil
}

func (r *InadmissibleWorkloadRequeuer) NotifyClusterQueue(cqName kueue.ClusterQueueReference) {
	r.eventCh <- event.TypedGenericEvent[requeueRequest]{Object: requeueRequest{ClusterQueue: cqName}}
}

func (r *InadmissibleWorkloadRequeuer) NotifyCohort(cohortName kueue.CohortReference) {
	r.eventCh <- event.TypedGenericEvent[requeueRequest]{Object: requeueRequest{Cohort: cohortName}}
}

type inadmissibleHandler struct{}

func (h *inadmissibleHandler) Create(context.Context, event.TypedCreateEvent[requeueRequest], workqueue.TypedRateLimitingInterface[requeueRequest]) {
}
func (h *inadmissibleHandler) Update(context.Context, event.TypedUpdateEvent[requeueRequest], workqueue.TypedRateLimitingInterface[requeueRequest]) {
}
func (h *inadmissibleHandler) Delete(context.Context, event.TypedDeleteEvent[requeueRequest], workqueue.TypedRateLimitingInterface[requeueRequest]) {
}
func (h *inadmissibleHandler) Generic(_ context.Context, e event.TypedGenericEvent[requeueRequest], q workqueue.TypedRateLimitingInterface[requeueRequest]) {
	q.AddAfter(e.Object, batchPeriod)
}

func (r *InadmissibleWorkloadRequeuer) SetupWithManager(mgr ctrl.Manager) error {
	return builder.TypedControllerManagedBy[requeueRequest](mgr).
		Named("inadmissible_workload_controller").
		WatchesRawSource(source.TypedChannel(r.eventCh, &inadmissibleHandler{})).
		Complete(r)
}
