package common

import (
	"context"

	virtv1 "kubevirt.io/client-go/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetVirtualMachines -
func GetVirtualMachines(r ReconcilerCommon, namespace string, labelSelectorMap map[string]string) (*virtv1.VirtualMachineList, error) {
	virtualMachineList := &virtv1.VirtualMachineList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelectorMap) > 0 {
		labels := client.MatchingLabels(labelSelectorMap)
		listOpts = append(listOpts, labels)
	}

	err := r.GetClient().List(context.TODO(), virtualMachineList, listOpts...)
	if err != nil {
		return virtualMachineList, err
	}

	return virtualMachineList, nil
}

// GetVirtualMachineInstances -
func GetVirtualMachineInstances(r ReconcilerCommon, namespace string, labelSelectorMap map[string]string) (*virtv1.VirtualMachineInstanceList, error) {
	virtualMachineInstanceList := &virtv1.VirtualMachineInstanceList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelectorMap) > 0 {
		labels := client.MatchingLabels(labelSelectorMap)
		listOpts = append(listOpts, labels)
	}

	err := r.GetClient().List(context.TODO(), virtualMachineInstanceList, listOpts...)
	if err != nil {
		return virtualMachineInstanceList, err
	}

	return virtualMachineInstanceList, nil
}
