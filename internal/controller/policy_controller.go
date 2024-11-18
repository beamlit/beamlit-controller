/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	authorizationv1alpha1 "github.com/beamlit/beamlit-controller/api/v1alpha1/authorization"
	"github.com/beamlit/beamlit-controller/internal/beamlit"
	"github.com/beamlit/beamlit-controller/internal/controller/helper"
)

// PolicyReconciler reconciles a Policy object
type PolicyReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	BeamlitClient   *beamlit.Client
	ManagedPolicies map[string]ManagedPolicyRef // key: policyName
}

type ManagedPolicyRef struct {
	lastGeneratedID int64
	namespacedName  types.NamespacedName
}

const policyFinalizer = "policy.beamlit.com/finalizer"

//+kubebuilder:rbac:groups=authorization.beamlit.com,resources=policies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=authorization.beamlit.com,resources=policies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=authorization.beamlit.com,resources=policies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Policy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(0).Info("Reconciling Policy", "Name", req.NamespacedName)
	var policy authorizationv1alpha1.Policy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			logger.V(0).Info("Policy not found", "Name", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		logger.V(0).Error(err, "Failed to get Policy")
		return ctrl.Result{}, err
	}

	if policy.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&policy, policyFinalizer) {
			logger.V(0).Info("Finalizing Policy", "Name", policy.Name)
			if err := r.finalizePolicy(ctx, &policy); err != nil {
				logger.V(0).Error(err, "Failed to finalize Policy")
				return ctrl.Result{}, err
			}
			delete(r.ManagedPolicies, policy.Name)
			controllerutil.RemoveFinalizer(&policy, policyFinalizer)
			if err := r.Update(ctx, &policy); err != nil {
				logger.V(0).Error(err, "Failed to update Policy")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&policy, policyFinalizer) {
		logger.V(0).Info("Adding finalizer to Policy", "Name", policy.Name)
		controllerutil.AddFinalizer(&policy, policyFinalizer)
		if err := r.Update(ctx, &policy); err != nil {
			logger.V(0).Error(err, "Failed to update Policy")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.createOrUpdate(ctx, &policy); err != nil {
		if errors.IsConflict(err) {
			logger.V(0).Info("Conflict detected, retrying", "error", err)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.V(0).Error(err, "Failed to create or update Policy")
		if err := r.Client.Status().Update(ctx, &policy); err != nil {
			logger.V(0).Error(err, "Failed to update Policy status")
			return ctrl.Result{}, err
		}
		r.ManagedPolicies[policy.Name] = ManagedPolicyRef{
			lastGeneratedID: policy.Generation,
			namespacedName:  req.NamespacedName,
		}
		return ctrl.Result{}, err
	}
	logger.V(0).Info("Successfully created or updated Policy", "Name", policy.Name)
	return ctrl.Result{}, nil

}

func (r *PolicyReconciler) createOrUpdate(ctx context.Context, policy *authorizationv1alpha1.Policy) error {
	ref, ok := r.ManagedPolicies[policy.Name]
	if ok {
		if ref.namespacedName.Namespace != policy.Namespace || ref.namespacedName.Name != policy.Name {
			return fmt.Errorf("policy %s is already defined", policy.Name)
		}
	}
	if ref.lastGeneratedID == policy.Generation {
		return nil
	}
	beamlitPolicy, err := r.BeamlitClient.CreateOrUpdatePolicy(ctx, *helper.ToBeamlitPolicy(policy))
	if err != nil {
		return err
	}
	policy.Status.Workspace = *beamlitPolicy.Workspace
	return nil
}

func (r *PolicyReconciler) finalizePolicy(ctx context.Context, policy *authorizationv1alpha1.Policy) error {
	return r.BeamlitClient.DeletePolicy(ctx, policy.Name)
}

// SetupWithManager sets up the controller with the Manager.
func (r *PolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&authorizationv1alpha1.Policy{}).
		Complete(r)
}
