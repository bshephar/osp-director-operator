/*


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

// Generated by:
//
// operator-sdk create webhook --group osp-director --version v1beta1 --kind OpenStackBaremetalSet --programmatic-validation
//

package v1beta1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var baremetalsetlog = logf.Log.WithName("baremetalset-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *OpenStackBaremetalSet) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-osp-director-openstack-org-v1beta1-openstackbaremetalset,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackbaremetalsets,versions=v1beta1,name=vopenstackbaremetalset.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &OpenStackBaremetalSet{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackBaremetalSet) ValidateCreate() error {
	baremetalsetlog.Info("validate create", "name", r.Name)

	if err := checkBackupOperationBlocksAction(r.Namespace, APIActionCreate); err != nil {
		return err
	}

	//
	// validate that for all configured subnets an osnet exists
	//
	if err := validateNetworks(r.GetNamespace(), r.Spec.Networks); err != nil {
		return err
	}

	return r.validateCr()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackBaremetalSet) ValidateUpdate(old runtime.Object) error {
	baremetalsetlog.Info("validate update", "name", r.Name)

	//
	// validate that for all configured subnets an osnet exists
	//
	if err := validateNetworks(r.GetNamespace(), r.Spec.Networks); err != nil {
		return err
	}

	return r.validateCr()

}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackBaremetalSet) ValidateDelete() error {
	baremetalsetlog.Info("validate delete", "name", r.Name)

	return checkBackupOperationBlocksAction(r.Namespace, APIActionDelete)
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstackbaremetalset,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackbaremetalsets,verbs=create;update,versions=v1beta1,name=mopenstackbaremetalset.kb.io,admissionReviewVersions={v1,v1beta1}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackBaremetalSet) Default() {
	baremetalsetlog.Info("default", "name", r.Name)

	//
	// set OpenStackNetConfig reference label if not already there
	// Note, any rename of the osnetcfg won't be reflected
	//
	if _, ok := r.GetLabels()[OpenStackNetConfigReconcileLabel]; !ok {
		labels, err := AddOSNetConfigRefLabel(
			r.Namespace,
			r.Spec.Networks[0],
			r.DeepCopy().GetLabels(),
		)
		if err != nil {
			baremetalsetlog.Error(err, fmt.Sprintf("error adding OpenStackNetConfig reference label on %s - %s: %s", r.Kind, r.Name, err))
		}

		r.SetLabels(labels)
		baremetalsetlog.Info(fmt.Sprintf("%s %s labels set to %v", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.GetLabels()))
	}

	//
	// add labels of all networks used by this CR
	//
	labels := AddOSNetNameLowerLabels(
		baremetalsetlog,
		r.DeepCopy().GetLabels(),
		r.Spec.Networks,
	)
	if !equality.Semantic.DeepEqual(
		labels,
		r.GetLabels(),
	) {
		r.SetLabels(labels)
		baremetalsetlog.Info(fmt.Sprintf("%s %s labels set to %v", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.GetLabels()))
	}

}

func (r *OpenStackBaremetalSet) validateCr() error {
	if err := r.checkBaseImageReqs(); err != nil {
		return err
	}

	if err := checkRoleNameExists(r.TypeMeta, r.ObjectMeta, r.Spec.RoleName); err != nil {
		return err
	}

	return nil
}
func (r *OpenStackBaremetalSet) checkBaseImageReqs() error {
	if r.Spec.BaseImageURL == "" && r.Spec.ProvisionServerName == "" {
		return fmt.Errorf("either \"baseImageUrl\" or \"provisionServerName\" must be provided")
	}

	return nil
}
