package integration

import (
	"context"

	"shellbox/internal/test"
)

// checkVMs lists all VMs in the resource group
func checkVMs(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	vmPager := env.Clients.ComputeClient.NewListPager(resourceGroupName, nil)
	for vmPager.More() {
		page, err := vmPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, vm := range page.Value {
			if vm.ID != nil {
				resources = append(resources, "VM: "+*vm.ID)
			}
		}
	}
	return resources
}

// checkNICs lists all NICs in the resource group
func checkNICs(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	nicPager := env.Clients.NICClient.NewListPager(resourceGroupName, nil)
	for nicPager.More() {
		page, err := nicPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, nic := range page.Value {
			if nic.ID != nil {
				resources = append(resources, "NIC: "+*nic.ID)
			}
		}
	}
	return resources
}

// checkNSGs lists all NSGs in the resource group
func checkNSGs(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	nsgPager := env.Clients.NSGClient.NewListPager(resourceGroupName, nil)
	for nsgPager.More() {
		page, err := nsgPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, nsg := range page.Value {
			if nsg.ID != nil {
				resources = append(resources, "NSG: "+*nsg.ID)
			}
		}
	}
	return resources
}

// checkVNets lists all VNets in the resource group
func checkVNets(ctx context.Context, env *test.Environment, resourceGroupName string) []string {
	var resources []string
	vnetPager := env.Clients.NetworkClient.NewListPager(resourceGroupName, nil)
	for vnetPager.More() {
		page, err := vnetPager.NextPage(ctx)
		if err != nil {
			return resources
		}
		for _, vnet := range page.Value {
			if vnet.ID != nil {
				resources = append(resources, "VNet: "+*vnet.ID)
			}
		}
	}
	return resources
}

// Note: Resource group verification is now handled by TestMain in main_test.go
// which runs the verification both before and after all tests
