/*
Copyright 2021.

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

// This controller is based heavily on https://gist.github.com/esys/e9214229d175c265f37913e19def5dcc

package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	tracev1alpha1 "github.com/iovisor/kubectl-trace/operator/api/v1alpha1"
	"github.com/iovisor/kubectl-trace/operator/metrics"
)

// TraceJobGCReconciler reconciles a TraceJobGC object
type TraceJobGCReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=trace.iovisor.org,resources=tracejobs,verbs=get;list;delete
// +kubebuilder:rbac:groups=trace.iovisor.org,resources=tracejobgcs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=trace.iovisor.org,resources=tracejobgcs/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.6.4/pkg/reconcile
func (r *TraceJobGCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("tracejobgc", req.NamespacedName)

	garbageCollector := &tracev1alpha1.TraceJobGC{}
	if err := r.Get(ctx, req.NamespacedName, garbageCollector); err != nil {
		log.Error(err, "unable to fetch TraceJobGC config", "Request", req)
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	garbageCollector.Status.LastStarted = metav1.Time{Time: time.Now()}

	var tracejobs tracev1alpha1.TraceJobList
	if err := r.List(ctx, &tracejobs, client.InNamespace(garbageCollector.Namespace)); err != nil {
		log.Error(err, "unable to list Tracejobs", "Namespace", garbageCollector.Namespace)
		metrics.TracejobGCTracejobListErrorTotal.Inc()
		return ctrl.Result{}, err
	}

	succeededCount := 0.0
	failedCount := 0.0
	invalidCount := 0.0
	runningCount := 0.0

	for _, tj := range tracejobs.Items {
		finished, finishedType, finishedTime := r.isTraceJobFinished(tj)
		if finished {
			if !r.deleteTraceJobIfExpired(ctx, &tj, finishedType, finishedTime, *garbageCollector) {
				if finishedType == tracev1alpha1.ConditionTypeSuccess {
					succeededCount += 1
				} else if finishedType == tracev1alpha1.ConditionTypeFailed {
					failedCount += 1
				} else if finishedType == tracev1alpha1.ConditionTypeInvalid {
					invalidCount += 1
				}
			}
		} else {
			runningCount += 1
		}
	}

	metrics.TracejobGCTracejobJobsCount.WithLabelValues(string(tracev1alpha1.ConditionTypeSuccess)).Set(succeededCount)
	metrics.TracejobGCTracejobJobsCount.WithLabelValues(string(tracev1alpha1.ConditionTypeFailed)).Set(failedCount)
	metrics.TracejobGCTracejobJobsCount.WithLabelValues(string(tracev1alpha1.ConditionTypeInvalid)).Set(invalidCount)
	metrics.TracejobGCTracejobJobsCount.WithLabelValues(string(tracev1alpha1.ConditionTypeRunning)).Set(runningCount)

	garbageCollector.Status.LastFinished = metav1.Time{Time: time.Now()}

	if err := r.Status().Update(ctx, garbageCollector); err != nil {
		log.Error(err, "unable to update TraceJobGC status")
		metrics.TracejobGCStatusUpdateErrorTotal.Inc()
		return ctrl.Result{}, err
	}

	gcDuration := garbageCollector.Status.LastFinished.Time.Sub(garbageCollector.Status.LastStarted.Time)
	metrics.GCDurationMilliseconds.Observe(float64(gcDuration.Milliseconds()))

	metrics.TracejobGCExecutedTotal.Inc()
	return ctrl.Result{RequeueAfter: time.Duration(garbageCollector.Spec.Frequency) * time.Second}, nil
}

func (r *TraceJobGCReconciler) isTraceJobFinished(tj tracev1alpha1.TraceJob) (bool, tracev1alpha1.ConditionType, time.Time) {
	for _, c := range tj.Status.Conditions {
		if (c.Type == tracev1alpha1.ConditionTypeSuccess || c.Type == tracev1alpha1.ConditionTypeFailed || c.Type == tracev1alpha1.ConditionTypeInvalid) && c.Status == tracev1alpha1.ConditionStatusTrue {
			return true, c.Type, c.LastTransitionTime.Time
		}
	}
	return false, "", time.Time{}
}

func (r *TraceJobGCReconciler) deleteTraceJobIfExpired(ctx context.Context, tj *tracev1alpha1.TraceJob, finishedType tracev1alpha1.ConditionType, statusTime time.Time, garbageCollector tracev1alpha1.TraceJobGC) bool {
	log := r.Log.WithValues("TraceJobGC", garbageCollector.Namespace, "TraceJob", tj.Name, "Finished", statusTime)

	elapsed := time.Since(statusTime)
	if finishedType == tracev1alpha1.ConditionTypeSuccess && int64(elapsed.Seconds()) > garbageCollector.Spec.CompletedTTL {
		if err := r.Delete(ctx, tj, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			metrics.TracejobGCTracejobDeleteErrorTotal.WithLabelValues(string(tracev1alpha1.ConditionTypeSuccess)).Inc()
			log.Error(err, "unable to delete completed TraceJob", "Elapsed", elapsed)
			return false
		} else {
			metrics.TracejobGCTracejobDeleteTotal.WithLabelValues(string(tracev1alpha1.ConditionTypeSuccess)).Inc()
			log.Info("deleted expired successful TraceJob", "Elapsed", elapsed)
			return true
		}
	} else if finishedType == tracev1alpha1.ConditionTypeFailed && int64(elapsed.Seconds()) > garbageCollector.Spec.FailedTTL {
		if err := r.Delete(ctx, tj, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			metrics.TracejobGCTracejobDeleteErrorTotal.WithLabelValues(string(tracev1alpha1.ConditionTypeFailed)).Inc()
			log.Error(err, "unable to delete failed TraceJob", "Elapsed", elapsed)
			return false
		} else {
			metrics.TracejobGCTracejobDeleteTotal.WithLabelValues(string(tracev1alpha1.ConditionTypeFailed)).Inc()
			log.Info("deleted expired failed TraceJob", "Elapsed", elapsed)
			return true
		}
	} else if finishedType == tracev1alpha1.ConditionTypeInvalid && int64(elapsed.Seconds()) > garbageCollector.Spec.InvalidTTL {
		if err := r.Delete(ctx, tj, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil {
			metrics.TracejobGCTracejobDeleteErrorTotal.WithLabelValues(string(tracev1alpha1.ConditionTypeInvalid)).Inc()
			log.Error(err, "unable to delete invalid TraceJob", "Elapsed", elapsed)
			return false
		} else {
			metrics.TracejobGCTracejobDeleteTotal.WithLabelValues(string(tracev1alpha1.ConditionTypeInvalid)).Inc()
			log.Info("deleted expired invalid TraceJob", "Elapsed", elapsed)
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *TraceJobGCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&tracev1alpha1.TraceJobGC{}).
		Complete(r)
}
