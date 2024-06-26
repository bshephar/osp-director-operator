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

package v1beta1

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	nmstate "github.com/openstack-k8s-operators/osp-director-operator/pkg/nmstate"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var openstacknetconfiglog = logf.Log.WithName("openstacknetconfig-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *OpenStackNetConfig) SetupWebhookWithManager(mgr ctrl.Manager) error {

	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstacknetconfig,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstacknetconfigs,verbs=create;update,versions=v1beta1,name=mopenstacknetconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackNetConfig{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackNetConfig) Default() {
	openstacknetconfiglog.Info("default", "name", r.Name)

	for netIdx := range r.Spec.Networks {
		//
		// Auto flag IsControlPlane if Name and NameLower matches ControlPlane network names
		//
		if r.Spec.Networks[netIdx].Name == ControlPlaneName && r.Spec.Networks[netIdx].NameLower == ControlPlaneNameLower {
			r.Spec.Networks[netIdx].IsControlPlane = true
		}

		//
		// The OpenStackNetConfig "XYZ" is invalid: spec.routes: Invalid value: "null": spec.routes in body must be of type array: "null"
		//
		for subnetIdx := range r.Spec.Networks[netIdx].Subnets {
			if r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv4.Routes == nil {
				r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv4.Routes = []Route{}
			}
			if r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv6.Routes == nil {
				r.Spec.Networks[netIdx].Subnets[subnetIdx].IPv6.Routes = []Route{}
			}
		}
	}

	//
	// set default PhysNetworks name/prefix if non speficied
	//
	if len(r.Spec.OVNBridgeMacMappings.PhysNetworks) == 0 {
		r.Spec.OVNBridgeMacMappings.PhysNetworks = []Physnet{
			{
				Name:      DefaultOVNChassisPhysNetName,
				MACPrefix: DefaultOVNChassisPhysNetMACPrefix,
			},
		}
	}

	if r.Spec.Reservations == nil {
		r.Spec.Reservations = map[string]OpenStackNetStaticNodeReservations{}
	} else {
		for node, res := range r.Spec.Reservations {
			if res.IPReservations == nil {
				res.IPReservations = map[string]string{}
			}
			if res.MACReservations == nil {
				res.MACReservations = map[string]string{}
			}

			r.Spec.Reservations[node] = res
		}
	}

	if r.Spec.PreserveReservations == nil {
		var trueVal bool = true
		r.Spec.PreserveReservations = &trueVal
	}

	//
	// The default domain name if non specified
	//
	if r.Spec.DomainName == "" {
		r.Spec.DomainName = DefaultDomainName
	}

	//
	//  spec.dnsServers in body must be of type array
	//
	if r.Spec.DNSServers == nil {
		r.Spec.DNSServers = []string{}
	}

	//
	//  spec.dnsSearchDomains in body must be of type array
	//
	if r.Spec.DNSSearchDomains == nil {
		r.Spec.DNSSearchDomains = []string{}
	}
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-osp-director-openstack-org-v1beta1-openstacknetconfig,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstacknetconfigs,verbs=create;update,versions=v1beta1,name=vopenstacknetconfig.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackNetConfig{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetConfig) ValidateCreate() error {
	openstacknetconfiglog.Info("validate create", "name", r.Name)

	//
	// Verify that network definitions
	//
	err := r.validateNetworks()
	if err != nil {
		return err
	}

	//
	// Verify that the specified control plane network name and name_lower match the expected ooo names
	//
	err = r.validateControlPlaneNetworkNames()
	if err != nil {
		return err
	}

	//
	// Validate static IP address reservations
	//
	err = r.validateStaticIPReservations()
	if err != nil {
		return err
	}

	//
	// Validate static MAC address reservations
	//
	err = r.validateStaticMacReservations(nil)
	if err != nil {
		return err
	}

	//
	// Validate domainName, must include a top-level domain and at least one subdomain
	//
	if err := checkDomainName(r.Spec.DomainName); err != nil {
		return err
	}

	return CheckBackupOperationBlocksAction(r.Namespace, shared.APIActionCreate)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetConfig) ValidateUpdate(old runtime.Object) error {
	openstacknetconfiglog.Info("validate update", "name", r.Name)

	// Get the OpenStackNetConfig object
	var ok bool
	var oldInstance *OpenStackNetConfig

	if oldInstance, ok = old.(*OpenStackNetConfig); !ok {
		return fmt.Errorf("runtime object is not an OpenStackNetConfig")
	}

	//
	// validate that the bridge names won't change on CR update
	//
	err := r.validateBridgeNameChanged(oldInstance)
	if err != nil {
		return err
	}

	//
	// Validate static IP address reservations
	//
	err = r.validateStaticIPReservations()
	if err != nil {
		return err
	}

	//
	// Validate static MAC address reservations
	//
	err = r.validateStaticMacReservations(oldInstance)
	if err != nil {
		return err
	}

	if r.Spec.DomainName != oldInstance.Spec.DomainName {
		return fmt.Errorf("domainName cannot be modified")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackNetConfig) ValidateDelete() error {
	openstacknetconfiglog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return CheckBackupOperationBlocksAction(r.Namespace, shared.APIActionDelete)
}

// validateControlPlaneNetworkNames - validate that the specified control plane network name and name_lower match the expected ooo names
func (r *OpenStackNetConfig) validateControlPlaneNetworkNames() error {
	// Verify that the specified control plane network name and name_lower match the expected ooo names
	for _, net := range r.Spec.Networks {
		if net.IsControlPlane {
			if net.Name != ControlPlaneName {
				return fmt.Errorf("control plane network name %s does not match %s", net.Name, ControlPlaneName)
			}
			if net.NameLower != ControlPlaneNameLower {
				return fmt.Errorf("control plane network name_lower  %s does not match %s", net.NameLower, ControlPlaneNameLower)
			}
		}
	}

	return nil
}

// validateBridgeNameChanged - validate that the bridge names won't change on CR update
func (r *OpenStackNetConfig) validateBridgeNameChanged(oldInstance *OpenStackNetConfig) error {
	for attachRef, attachCfg := range r.Spec.AttachConfigurations {

		// if the attachRef is in the spec of the old CR instance:
		// * check if bridge names did not change
		// * otherwise we expect it to be a new attachconfiguration/interface to configure on the workers.
		if _, ok := oldInstance.Spec.AttachConfigurations[attachRef]; ok {
			// Get the current (potentially new) bridge name, if any
			curBridge, err := nmstate.GetDesiredStateBridgeName(attachCfg.NodeNetworkConfigurationPolicy.DesiredState.Raw)

			if err != nil {
				return err
			}

			oldBridge, err := nmstate.GetDesiredStateBridgeName(oldInstance.Spec.AttachConfigurations[attachRef].NodeNetworkConfigurationPolicy.DesiredState.Raw)

			if err != nil {
				return err
			}

			if curBridge != oldBridge {
				return fmt.Errorf("bridge names may not be changed")
			}
		}
	}

	return nil
}

// validateStaticMacReservations - validate static MAC address reservations
func (r *OpenStackNetConfig) validateStaticMacReservations(oldInstance *OpenStackNetConfig) error {
	// fill an empty reservations map to check for uniq MAC reservations
	reservations := map[string]OpenStackMACNodeReservation{}

	for node, res := range r.Spec.Reservations {
		for physnet, mac := range res.MACReservations {
			//
			// check if the MAC address has a valid format
			//
			if _, err := net.ParseMAC(mac); err != nil {
				return fmt.Errorf("MAC address %s of node %s has an invalid format", mac, node)
			}

			//
			// check for duplicate reservations on static reservations
			//
			if !IsUniqMAC(reservations, mac) {
				return fmt.Errorf("MAC address %s of node %s is not uniq", mac, node)
			}

			//
			// check that a MAC reservation won't change
			//
			if oldInstance != nil {
				if currentMAC, ok := oldInstance.Spec.Reservations[node].MACReservations[physnet]; ok && currentMAC != mac {
					return fmt.Errorf("MAC address %s of node %s must not change - new MAC address %s", currentMAC, node, mac)
				}
			}
		}

		// if all tests pass add to reservations
		reservations[node] = OpenStackMACNodeReservation{
			Reservations: res.MACReservations,
		}
	}

	return nil
}

// validateStaticIPReservations - validate static IP address reservations
func (r *OpenStackNetConfig) validateStaticIPReservations() error {
	// fill an empty reservations map to check for uniq IP reservations
	reservations := map[string]string{}

	//
	// Create nested map with per net, ip -> node name reservations
	//
	osNetList := &OpenStackNetList{}

	listOpts := []client.ListOption{
		client.InNamespace(r.Namespace),
	}

	if err := webhookClient.List(context.TODO(), osNetList, listOpts...); err != nil {
		return err
	}

	netReservations := map[string]map[string]string{}
	for _, osNet := range osNetList.Items {
		if netReservations[osNet.Spec.NameLower] == nil {
			netReservations[osNet.Spec.NameLower] = map[string]string{}
		}
		for node, res := range osNet.Status.Reservations {
			netReservations[osNet.Spec.NameLower][res.IP] = node
		}
	}

	//
	//  verify all the Spec.Reservations provided
	//
	for node, res := range r.Spec.Reservations {
		for netName, resIP := range res.IPReservations {
			//
			// check if the IP address has a valid format
			//
			ip := net.ParseIP(resIP)
			if ip == nil {
				return fmt.Errorf("IP address %s of node %s has an invalid format", resIP, node)
			}

			//
			// check if IP matches osnet spec
			//
			for _, osNet := range r.Spec.Networks {
				for _, subnet := range osNet.Subnets {
					if subnet.Name == netName {
						var ipnet *net.IPNet
						if subnet.IPv4.Cidr != "" {
							_, ipnet, _ = net.ParseCIDR(subnet.IPv4.Cidr)
						} else {
							_, ipnet, _ = net.ParseCIDR(subnet.IPv6.Cidr)
						}

						if !ipnet.Contains(ip) {
							return fmt.Errorf("IP address %s of node %s conflicts with subnet %s definition %s",
								resIP,
								node,
								netName,
								ipnet.String(),
							)
						}
					}
				}
			}

			//
			// check for duplicate reservations on static reservations
			//
			if resNode, ok := reservations[resIP]; ok && resNode != node {
				return fmt.Errorf("IP address %s of node %s is not uniq. Already used by %s",
					resIP,
					node,
					resNode,
				)
			}

			//
			// check for duplicate reservations on all active reservations
			//
			if resNode, ok := netReservations[netName][resIP]; ok && resNode != node {
				return fmt.Errorf("IP address %s of node %s is not uniq. Already used by %s",
					resIP,
					node,
					resNode,
				)
			}

			// if all tests pass add to reservations
			reservations[resIP] = node
		}
	}

	return nil
}

func checkDomainName(domainName string) error {

	// TODO: implement the same validation as freeipa validate_domain_name()
	//       in https://github.com/freeipa/freeipa/blob/master/ipalib/util.py
	if domainName != "" && len(strings.Split(domainName, ".")) < 2 {
		return fmt.Errorf("domainName must include a top-level domain and at least one subdomain")
	}
	return nil
}

// validateNetworks - validates the details provided for a networks definition
func (r *OpenStackNetConfig) validateNetworks() error {

	isCtlplaneNetwork := false

	for _, osnet := range r.Spec.Networks {
		if osnet.IsControlPlane {
			isCtlplaneNetwork = true
		}

		for _, subnet := range osnet.Subnets {
			//
			// A subnet can only have either IPv4 or IPv6 definition
			//
			if subnet.IPv4.Cidr != "" && subnet.IPv6.Cidr != "" {
				return fmt.Errorf("subnet %s can only be IPv4 or IPv6, not both", subnet.Name)
			}

			var ipnet *net.IPNet
			var err error
			subnetDetails := map[string]string{}
			if subnet.IPv4.Cidr != "" {
				var ip net.IP
				ip, ipnet, err = net.ParseCIDR(subnet.IPv4.Cidr)
				if err != nil {
					return err
				}

				// validate provided IP is correct IPv4
				if shared.IsIPv4(ip) {
					subnetDetails["AllocationStart"] = subnet.IPv4.AllocationStart
					subnetDetails["AllocationEnd"] = subnet.IPv4.AllocationEnd
					if subnet.IPv4.Gateway != "" {
						subnetDetails["Gateway"] = subnet.IPv4.Gateway
					}
				} else {
					return fmt.Errorf("%s is not a valid IPv4 cidr", subnet.IPv4.Cidr)
				}
			} else if subnet.IPv6.Cidr != "" {
				var ip net.IP
				ip, ipnet, err = net.ParseCIDR(subnet.IPv6.Cidr)
				if err != nil {
					return err
				}

				// validate provided IP is correct IPv6
				if shared.IsIPv6(ip) {
					subnetDetails["AllocationStart"] = subnet.IPv6.AllocationStart
					subnetDetails["AllocationEnd"] = subnet.IPv6.AllocationEnd
					if subnet.IPv6.Gateway != "" {
						subnetDetails["Gateway"] = subnet.IPv6.Gateway
					}

				} else {
					return fmt.Errorf("%s is not a valid IPv6 cidr", subnet.IPv4.Cidr)
				}
			} else {
				// we should never hit this as cidr is a required parameter
				return fmt.Errorf("either IPv4.Cidr or IPv6.Cidr must be provided %s", subnet.Name)
			}

			//
			// check if subnet AllocationStart, AllocationEnd and Gateway has
			// * a valid format
			// * are part of the specified subnet.Cird
			//
			for ipType, ipToTest := range subnetDetails {
				ip := net.ParseIP(ipToTest)
				if ip == nil {
					return fmt.Errorf("%s IP address %s of subnet %s has an invalid format",
						ipType,
						ipToTest,
						subnet.Name)
				}
				if !ipnet.Contains(ip) {
					return fmt.Errorf("%s IP address %s of subnet %s conflicts with cidr %s",
						ipType,
						ipToTest,
						subnet.Name,
						ipnet.String())
				}
			}
		}
	}

	//
	// if there is no ctlplane tagged network, return error
	//
	if !isCtlplaneNetwork {
		return fmt.Errorf("no network tagged as IsControlPlane")
	}

	return nil
}
