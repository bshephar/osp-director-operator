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
// operator-sdk create webhook --group osp-director --version v1beta1 --kind OpenStackControlPlane --programmatic-validation
//

package v1beta1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
)

// OpenStackControlPlaneDefaults -
type OpenStackControlPlaneDefaults struct {
	OpenStackClientImageURL string
	OpenStackRelease        string
}

var openstackControlPlaneDefaults OpenStackControlPlaneDefaults

// log is for logging in this package.
var controlplanelog = logf.Log.WithName("controlplane-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *OpenStackControlPlane) SetupWebhookWithManager(mgr ctrl.Manager, defaults OpenStackControlPlaneDefaults) error {

	openstackControlPlaneDefaults = defaults

	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-osp-director-openstack-org-v1beta1-openstackcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackcontrolplanes,versions=v1beta1,name=vopenstackcontrolplane.kb.io,admissionReviewVersions={v1,v1beta1}

var _ webhook.Validator = &OpenStackControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateCreate() error {
	controlplanelog.Info("validate create", "name", r.Name)

	controlPlaneList := &OpenStackControlPlaneList{}

	listOpts := []client.ListOption{
		client.InNamespace(r.Namespace),
	}

	err := webhookClient.List(context.TODO(), controlPlaneList, listOpts...)

	if err != nil {
		return err
	}

	if len(controlPlaneList.Items) >= 1 {
		return fmt.Errorf("only one OpenStackControlPlane instance is supported at this time")
	}

	err = checkDomainName(r.Spec.DomainName)

	if err != nil {
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateUpdate(old runtime.Object) error {
	controlplanelog.Info("validate update", "name", r.Name)

	oldControlPlane := old.(*OpenStackControlPlane)
	if r.Spec.DomainName != oldControlPlane.Spec.DomainName {
		return fmt.Errorf("domainName cannot be modified")
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateDelete() error {
	controlplanelog.Info("validate delete", "name", r.Name)

	return nil
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstackcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=mopenstackcontrolplane.kb.io,admissionReviewVersions={v1,v1beta1}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackControlPlane) Default() {
	openstackephemeralheatlog.Info("default", "name", r.Name)

	if r.Spec.OpenStackClientImageURL == "" {
		r.Spec.OpenStackClientImageURL = openstackControlPlaneDefaults.OpenStackClientImageURL
	}

	//
	// set default PhysNetworks name/prefix if non speficied
	//
	if len(r.Spec.PhysNetworks) == 0 {
		r.Spec.PhysNetworks = []Physnet{
			{
				Name:      controlplane.DefaultOVNChassisPhysNetName,
				MACPrefix: controlplane.DefaultOVNChassisPhysNetMACPrefix,
			},
		}
	}

	//
	// set default OpenStackRelease
	//
	if r.Spec.OpenStackRelease == "" {
		r.Spec.OpenStackRelease = openstackControlPlaneDefaults.OpenStackRelease
	}

}
