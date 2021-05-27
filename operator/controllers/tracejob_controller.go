package controllers

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/iovisor/kubectl-trace/operator/api/v1alpha1"
	"github.com/iovisor/kubectl-trace/operator/metrics"
	"github.com/iovisor/kubectl-trace/pkg/meta"
	"github.com/iovisor/kubectl-trace/pkg/tracejob"
)

// TraceJobReconciler reconciles a TraceJob object
type TraceJobReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=trace.iovisor.org,resources=tracejobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=trace.iovisor.org,resources=tracejobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;create;update

func (r *TraceJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("tracejob", req.NamespacedName)
	log.V(2).Info("Reconciling started")

	var tracejob v1alpha1.TraceJob
	if err := r.Get(ctx, req.NamespacedName, &tracejob); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Don't reconcile if we don't need to
	for _, c := range tracejob.Status.Conditions {
		if c.Type != v1alpha1.ConditionTypeRunning {
			return ctrl.Result{}, nil
		}
	}

	// set the artifact path if it's not already set on the tracejob
	if tracejob.Status.ArtifactPath == "" {
		artifactPath, err := r.artifactPath(tracejob)
		if err != nil {
			log.Error(err, "failed to set artifact path")
			metrics.TracejobOutputInvalidErrorTotal.WithLabelValues(tracejob.Spec.TargetNamespace).Inc()
			return ctrl.Result{}, err
		}
		tracejob.Status.ArtifactPath = artifactPath
	}

	if tracejob.Status.JobName == "" {
		target, err := r.resolveTraceJobTarget(tracejob)

		if err != nil {
			condition := v1alpha1.StatusCondition{
				Type:    v1alpha1.ConditionTypeInvalid,
				Message: err.Error(),
				Reason:  "InvalidTarget",
				Status:  v1alpha1.ConditionStatusTrue,
			}
			r.updateStatus(ctx, &tracejob, condition)
			log.Info(fmt.Sprintf("TraceJob had an invalid target with status %v", tracejob.Status))
			metrics.TracejobTargetInvalidErrorTotal.WithLabelValues(tracejob.Spec.TargetNamespace).Inc() // TODO check that we see this incremented in our integration test?
			return ctrl.Result{}, err
		}

		// attempt to create the tracejob
		job, err := r.createJob(ctx, tracejob, target)
		if err != nil {
			log.Error(err, "error creating job")
			metrics.TracejobCreateErrorTotal.WithLabelValues(tracejob.Spec.TargetNamespace).Inc()
			return ctrl.Result{}, err
		} else {
			tracejob.Status.JobName = job.ObjectMeta.Name
			condition := v1alpha1.StatusCondition{
				Type:    v1alpha1.ConditionTypeRunning,
				Message: "Running",
				Reason:  "New Job",
				Status:  v1alpha1.ConditionStatusTrue,
			}
			r.updateStatus(ctx, &tracejob, condition)
			log.Info(fmt.Sprintf("Created a new TraceJob with status %v", tracejob.Status))
			metrics.TracejobStartedTotal.WithLabelValues(tracejob.Spec.TargetNamespace).Inc()
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
	} else {
		// update the status of the existing tracejob
		namespacedName := types.NamespacedName{
			Name:      tracejob.Status.JobName,
			Namespace: tracejob.Namespace,
		}
		var job batchv1.Job
		if err := r.Get(ctx, namespacedName, &job); err != nil {
			return ctrl.Result{}, client.IgnoreNotFound(err)
		}

		for _, c := range job.Status.Conditions {
			if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
				r.updateStatus(ctx, &tracejob, v1alpha1.StatusCondition{
					Type:    v1alpha1.ConditionTypeSuccess,
					Status:  v1alpha1.ConditionStatusTrue,
					Reason:  "Successful",
					Message: fmt.Sprintf("The job %s completed successfully", tracejob.Status.JobName),
				})
				metrics.TracejobSucceededTotal.WithLabelValues(tracejob.Spec.TargetNamespace).Inc()
				return ctrl.Result{}, nil
			}
			if c.Type == batchv1.JobFailed && c.Status == corev1.ConditionTrue {
				r.updateStatus(ctx, &tracejob, v1alpha1.StatusCondition{
					Type:    v1alpha1.ConditionTypeFailed,
					Status:  v1alpha1.ConditionStatusTrue,
					Reason:  "Failed",
					Message: fmt.Sprintf("The job %s failed to complete successfully", tracejob.Status.JobName),
				})
				metrics.TracejobFailedTotal.WithLabelValues(tracejob.Spec.TargetNamespace).Inc()
				return ctrl.Result{}, nil
			}
		}

		condition := v1alpha1.StatusCondition{
			Status:  v1alpha1.ConditionStatusTrue,
			Type:    v1alpha1.ConditionTypeRunning,
			Message: "Waiting for job to complete",
		}

		log.V(2).Info(fmt.Sprintf("Reconcile of %s incomplete with status %v, requeueing", tracejob.Name, tracejob.Status))
		r.updateStatus(ctx, &tracejob, condition)
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}
}

func (r *TraceJobReconciler) resolveTraceJobTarget(job v1alpha1.TraceJob) (*tracejob.TraceJobTarget, error) {

	config, err := ctrl.GetConfig()
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	target, err := tracejob.ResolveTraceJobTarget(clientset, job.Spec.Resource, job.Spec.Container, job.Spec.TargetNamespace)
	if err != nil {
		return nil, err
	}

	return target, nil
}

func (r *TraceJobReconciler) artifactPath(tracejob v1alpha1.TraceJob) (string, error) {
	if tracejob.Spec.Output == "stdout" {
		return "stdout", nil
	}

	// check the URI format
	parsedUrl, err := url.Parse(tracejob.Spec.Output)
	if err != nil {
		return "", err
	}

	// check that we have a supported scheme
	if parsedUrl.Scheme != "gs" {
		return "", fmt.Errorf("unsupported scheme %s; upload paths must start with gs://", parsedUrl.Scheme)
	}

	traceGroup := tracejob.Spec.TargetNamespace
	if traceGroup == "" {
		if !strings.HasPrefix(tracejob.Spec.Resource, "node/") {
			return "", fmt.Errorf("target namespace must be specified or resource must be of type node")
		}
		traceGroup = tracejob.Spec.Resource
	}

	// compose the path for the bucket
	// see https://golang.org/pkg/time/#Time.Format for the formatting magic used here
	now := time.Now().UTC()
	datePart := now.Format("2006/01/02")
	namedPart := fmt.Sprintf("%s--%s--%s", tracejob.Spec.Tracer, now.Format("15-04-05"), string(tracejob.UID))

	output := strings.TrimSuffix(tracejob.Spec.Output, "/")

	return fmt.Sprintf("%s/%s/%s/%s", output, traceGroup, datePart, namedPart), nil
}

func (r *TraceJobReconciler) desiredJob(job v1alpha1.TraceJob, target *tracejob.TraceJobTarget) (*batchv1.Job, *corev1.ConfigMap, error) {
	tj := tracejob.TraceJob{
		Name:           fmt.Sprintf("%s%s", meta.ObjectNamePrefix, string(job.UID)),
		Namespace:      job.Namespace,
		ServiceAccount: "default", // TODO: what would be this?
		ID:             job.UID,
		Tracer:         job.Spec.Tracer,
		Target:         *target,
		Selector:       job.Spec.Selector, // TODO: what is this again from the job side?
		// FIXME this (above) should just be the ProcessSelector
		Output:              job.Status.ArtifactPath,
		Program:             job.Spec.Program,
		ProgramArgs:         job.Spec.ProgramArgs,
		ImageNameTag:        job.Spec.ImageNameTag,
		InitImageNameTag:    job.Spec.InitImageNameTag,
		FetchHeaders:        job.Spec.FetchHeaders,
		GoogleAppSecret:     job.Spec.GoogleAppSecret,
		Deadline:            job.Spec.Deadline,
		DeadlineGracePeriod: job.Spec.DeadlineGracePeriod,
	}

	tjob, tconfigmap := tj.Job(), tj.ConfigMap()

	err := ctrl.SetControllerReference(&job, tjob, r.Scheme)
	if err != nil {
		return nil, nil, err
	}

	err = ctrl.SetControllerReference(&job, tconfigmap, r.Scheme)
	if err != nil {
		return nil, nil, err
	}

	return tjob, tconfigmap, nil
}

func (r *TraceJobReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	pred := predicate.GenerationChangedPredicate{}
	return ctrl.NewControllerManagedBy(mgr).
		Owns(&batchv1.Job{}).
		Owns(&corev1.ConfigMap{}).
		For(&v1alpha1.TraceJob{}).
		WithEventFilter(pred).
		Complete(r)
}

func (r *TraceJobReconciler) createJob(ctx context.Context, tracejob v1alpha1.TraceJob, target *tracejob.TraceJobTarget) (*batchv1.Job, error) {
	job, configMap, err := r.desiredJob(tracejob, target)
	if err != nil {
		return nil, err
	}

	_, err = ctrl.CreateOrUpdate(ctx, r.Client, configMap,
		func() error {
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	_, err = ctrl.CreateOrUpdate(ctx, r.Client, job,
		func() error {
			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	return job, nil
}

func (r *TraceJobReconciler) updateStatus(ctx context.Context, tracejob *v1alpha1.TraceJob, condition v1alpha1.StatusCondition) error {
	v1alpha1.SetCondition(&tracejob.Status.Conditions, &condition)

	if err := r.Status().Update(ctx, tracejob); err != nil {
		if errors.IsConflict(err) {
			metrics.TracejobStatusUpdateConflictTotal.WithLabelValues(tracejob.Spec.TargetNamespace).Inc()
		} else {
			r.Log.Error(err, "failed to update status with unexpected error")
		}
		return err
	}
	return nil
}
