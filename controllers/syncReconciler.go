package controllers

import (
	"fmt"

	ocsv1 "github.com/openshift/ocs-operator/pkg/apis/ocs/v1"
	v1 "github.com/openshift/ocs-osd-deployer/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	controller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type SyncReconciler struct {
	ManagedOCSReconciler
	startReconcile chan struct{}
	doneReconcile  chan struct{}
}

func (r *SyncReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	fmt.Println("\n\tIn reconciler, waiting for signal.")
	<-r.startReconcile // Wait for reconcile to be requested.
	fmt.Println("\n\tRunning the reconcile")
	result, err := r.ManagedOCSReconciler.Reconcile(req)
	fmt.Println("\n\tThe reconcile is done, sending signal")
	r.doneReconcile <- struct{}{} // Signal that the reconcile has completed

	return result, err
}

func (r *SyncReconciler) RunReconcile() {
	fmt.Println("\n\tRunReconcile: Signaling recon")
	r.startReconcile <- struct{}{} // Signal Reconciler to start reconcile
	fmt.Println("\n\tRunReconcile: Waiting for recon")
	<-r.doneReconcile // Wait for the reconcile to complete
	fmt.Println("\n\tRunReconcile: Recon done")
}

func NewSyncReconciler(managedOCSReconciler ManagedOCSReconciler) SyncReconciler {
	return SyncReconciler{
		managedOCSReconciler,
		make(chan struct{}),
		make(chan struct{}),
	}
}

func (r *SyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctrlOptions := controller.Options{
		MaxConcurrentReconciles: 1,
	}
	managedOCSPredicates := builder.WithPredicates(
		predicate.GenerationChangedPredicate{},
	)
	addonParamsSecretWatchHandler := handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(
			func(obj handler.MapObject) []reconcile.Request {
				if obj.Meta.GetName() == r.AddonParamSecretName {
					return []reconcile.Request{{
						NamespacedName: types.NamespacedName{
							Name:      "managedocs",
							Namespace: obj.Meta.GetNamespace(),
						},
					}}
				}
				return nil
			},
		),
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(ctrlOptions).
		For(&v1.ManagedOCS{}, managedOCSPredicates).
		Owns(&ocsv1.StorageCluster{}).
		Watches(&source.Kind{Type: &corev1.Secret{}}, &addonParamsSecretWatchHandler).
		Complete(r)
}
