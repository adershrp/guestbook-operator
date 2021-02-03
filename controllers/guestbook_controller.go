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

package controllers

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	webappv1 "github.com/adershrp/guestbook-operator/api/v1"
)

// GuestBookReconciler reconciles a GuestBook object
type GuestBookReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=webapp.adershrp.org,resources=guestbooks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=webapp.adershrp.org,resources=guestbooks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=webapp.adershrp.org,resources=guestbooks/finalizers,verbs=update

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=list;watch;get;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=list;watch;get;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the GuestBook object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.7.0/pkg/reconcile
func (r *GuestBookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("guestbook", req.NamespacedName)
	log.Info("reconciling guestbook")

	var book webappv1.GuestBook
	if err := r.Get(ctx, req.NamespacedName, &book); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	var redis webappv1.Redis
	redisName := client.ObjectKey{Name: book.Spec.RedisName, Namespace: req.Namespace}
	if err := r.Get(ctx, redisName, &redis); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	deployment, err := r.desiredDeployment(book, redis)
	if err != nil {
		return ctrl.Result{}, err
	}

	svc, err := r.desiredService(book)
	if err != nil {
		return ctrl.Result{}, err
	}

	applyOpts := []client.PatchOption{client.ForceOwnership, client.FieldOwner("guestbook-controller")}

	err = r.Patch(ctx, &deployment, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Patch(ctx, &svc, client.Apply, applyOpts...)
	if err != nil {
		return ctrl.Result{}, err
	}

	book.Status.URL = urlForService(svc, book.Spec.Frontend.ServingPort)

	err = r.Status().Update(ctx, &book)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.Info("reconciled guestbook")
	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *GuestBookReconciler) SetupWithManager(mgr ctrl.Manager) error {

	mgr.GetFieldIndexer().IndexField(context.Background(),
		&webappv1.GuestBook{}, ".spec.redisName", func(obj client.Object) []string {
			redisName := obj.(*webappv1.GuestBook).Spec.RedisName
			if redisName == "" {
				return nil
			}
			return []string{redisName}
		})

	return ctrl.NewControllerManagedBy(mgr).
		For(&webappv1.GuestBook{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		// Watches(
		// 	&source.Kind{Type: &webappv1.Redis{}},
		// 	&handler.EnqueueRequestsFromMapFunc(&handler.),
		// ).
		Complete(r)
}
