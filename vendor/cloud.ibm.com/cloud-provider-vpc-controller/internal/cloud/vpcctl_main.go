/*******************************************************************************
* IBM Cloud Kubernetes Service, 5737-D43
* (C) Copyright IBM Corp. 2021 All Rights Reserved.
*
* SPDX-License-Identifier: Apache2.0
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*    http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*******************************************************************************/

package cloud

import (
	"errors"
	"fmt"

	"cloud.ibm.com/cloud-provider-vpc-controller/internal/ibm"
	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/klog"
	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpclb"
)

// Initialize logging to go to stdout
func init() {
	klog.SetOutputToStdout()
}

// CreateLoadBalancer - method gets called by the cloud provider EnsureLoadBalancer method.
//
// This method will get called when:
//  - the load balancer does not exist, initial create time
//  - the load balancer creation is still in progress, it will take ~5 min to create the VPC load balancer
//  - the Kubernetes load balancer service has been modified (new ports or other changes)
func CreateLoadBalancer(lbName, kubeService string) error {
	klog.Infof("Entering CreateLoadBalancer(%s, %s)", lbName, kubeService)
	if lbName == "" {
		klog.Errorf("Required argument is missing")
		return errors.New("Required argument is missing")
	}
	// Initialize Cloud VPC
	c, err := GetCloudProviderVpc()
	if err != nil {
		klog.Errorf("GetCloudProviderVpc failed: %v\n", err)
		return err
	}

	// If only 1 arg was passed in, then assume it is the kubeService name and not the lbName
	if kubeService == "" {
		kubeService = lbName
		lbName = ""
	}
	// Locate the correct Kubernetes service
	service, err := getKubeService(c.KubeClient, kubeService)
	if err != nil {
		klog.Errorf("getKubeService failed: %v\n", err)
		return err
	}
	// If we don't have a name for the VPC load balancer, generate one
	if lbName == "" {
		lbName = c.GenerateLoadBalancerName(service)
	}

	// Retrieve list of nodes
	nodes, err := getKubeNodes(c.KubeClient)
	if err != nil {
		klog.Errorf("getNodes failed: %v\n", err)
		return err
	}

	// Check to see if the VPC load balancer already exists
	lb, err := c.FindLoadBalancer(lbName, service)
	if err != nil {
		klog.Errorf("FindLoadBalancer failed: %v\n", err)
		return err
	}
	if lb == nil {
		// The specified VPC load balancer was not found.  Need to create it
		lb, err = c.CreateLoadBalancer(lbName, service, nodes)
		if err != nil {
			klog.Errorf("CreateLoadBalancer failed: %v\n", err)
			return err
		}

		// Log basic stats about the load balancer and return success (if the LB is READY or not NLB)
		// - return SUCCESS for non-NLB to remain backward compatibility, no additional operations need to be done
		// - don't return SUCCESS for NLB, because we can't do the DNS lookup of static IP if the LB is still pending
		if lb.IsReady() || !lb.IsNLB() {
			klog.Infof(lb.GetSummary())
			klog.Success(lb.GetSuccessString())
			return nil
		}
	}

	// Return basic stats about the load balancer for the cloud provider to log
	klog.Infof(lb.GetSummary())

	// If we get to this point, it means that EnsureLoadBalancer was called against a Load Balancer
	// that already exists. This is most likely due to a change is the Kubernetes service.
	// If the load balancer is not "Online/Active", there are no additional operations that can be performed.
	// In this case, we will return PENDING and the load balancer status
	if !lb.IsReady() {
		klog.Pending(lb.GetStatus())
		return nil
	}

	// The load balancer state is Online/Active.  This means that additional operations can be done.
	// Update the existing LB with any service or node changes that may have occurred.
	lb, err = c.UpdateLoadBalancer(lb, service, nodes)
	if err != nil {
		klog.Errorf("UpdateLoadBalancer failed: %v\n", err)
		return err
	}
	klog.Success(lb.GetSuccessString())
	return nil
}

// DeleteLoadBalancer - method gets called by the cloud provider EnsureLoadBalancerDeleted method.
//
// This method will get called when:
//  - the load balancer server is called to be deleted
//  - the load balancer deletion is still in progress, it will take ~5 min to delete the VPC load balancer
func DeleteLoadBalancer(lbName string) error {
	klog.Infof("Entering DeleteLoadBalancer(%s)", lbName)
	if lbName == "" {
		klog.Errorf("Required argument is missing")
		return errors.New("Required argument is missing")
	}

	// Initialize Cloud VPC
	c, err := GetCloudProviderVpc()
	if err != nil {
		klog.Errorf("GetCloudProviderVpc failed: %v\n", err)
		return err
	}

	// Check to see if the VPC load balancer already exists
	lb, err := c.FindLoadBalancer(lbName, nil)
	if err != nil {
		klog.Errorf("FindLoadBalancer failed: %v\n", err)
		return err
	}
	if lb == nil {
		// lbName did not resolve to a VPC LB, perhaps it is actually a Kube service
		service, err := getKubeService(c.KubeClient, lbName)
		if err == nil {
			lbName = c.GenerateLoadBalancerName(service)
			lb, err = c.FindLoadBalancer(lbName, service)
			if err != nil {
				klog.Errorf("FindLoadBalancer failed: %v\n", err)
				return err
			}
		}
	}

	if lb == nil {
		// The successful case is when we are called for a load balancer and that load balancer does not exist
		// Return "SUCCESS: Load balancer %s not found" to the cloud provider
		klog.Success("Load balancer %s not found", lbName)
		return nil
	}

	// Return basic stats about the load balancer for the cloud provider to log
	klog.Infof(lb.GetSummary())

	// Check the state of the load balancer to determine if the delete operation can even be attempted
	if !lb.IsReady() {
		klog.Pending(lb.GetStatus())
		return nil
	}

	// Attempt to delete the load balancer.
	err = c.DeleteLoadBalancer(lb, nil)
	if err != nil {
		klog.Errorf("DeleteLoadBalancer failed: %v\n", err)
		return err
	}

	// Since the delete operation was attempted, return SUCCESS since no additional operatons need to be done
	klog.Success("Delete operation started for load balancer: %s", lbName)
	return nil
}

// GetCloudProviderVpc - Retrieve cloud provider CloudVpc object
func GetCloudProviderVpc() (*vpclb.CloudVpc, error) {
	client, clusterID, err := cloudInit()
	if err != nil {
		return nil, err
	}
	cloud := ibm.Cloud{KubeClient: client, Config: &ibm.CloudConfig{Prov: ibm.Provider{ClusterID: clusterID}}}
	err = cloud.InitCloudVpc(shouldPrivateEndpointBeEnabled())
	if err != nil {
		return nil, err
	}
	return cloud.Vpc, nil
}

// MonitorLoadBalancers method gets called by the cloud provider monitorVpcLoadBalancers method.
// This routine will gather a list of all Kubernetes load balancers and return the status of the
// VPC load balancer associated with each one.
func MonitorLoadBalancers() error {
	klog.Infof("Entering MonitorLoadBalancers()")
	c, err := GetCloudProviderVpc()
	if err != nil {
		klog.Errorf("GetCloudProviderVpc failed: %v\n", err)
		return err
	}
	// Get list of Kubernetes services
	services, err := getKubeServices(c.KubeClient)
	if err != nil {
		klog.Errorf("getKubeServices failed: %v\n", err)
		return err
	}

	// Call the MonitorLoadBalancers function to get status of all VPC LBs
	lbMap, vpcMap, err := c.MonitorLoadBalancers(services)
	if err != nil {
		klog.Errorf("MonitorLoadBalancers failed: %v\n", err)
		return err
	}

	// Verify that we have a VPC LB for each of the Kube LB services
	for lbName, service := range lbMap {
		vpcLB, exists := vpcMap[lbName]
		if exists {
			klog.Infof("ServiceUID:%s Service:%s/%s %s",
				string(service.ObjectMeta.UID), service.ObjectMeta.Namespace, service.ObjectMeta.Name, vpcLB.GetSummary())
		} else {
			klog.NotFound("ServiceUID:%s Service:%s/%s Message:VPC load balancer not found for service",
				string(service.ObjectMeta.UID), service.ObjectMeta.Namespace, service.ObjectMeta.Name)
		}
	}

	klog.Infof("Exiting MonitorLoadBalancers()")
	return err
}

// StatusLoadBalancer - method gets called by the cloud provider GetLoadBalancer method
//
// The load balancer may be in the process of being created, updated, or deleted when this method is called.
// It is also possible that the load balancer will no longer even exist.
// Determine the state/existence of the load balancer and return the correct information to the cloud provider
func StatusLoadBalancer(lbName string) error {
	klog.Infof("Entering StatusLoadBalancer(%s)", lbName)
	if lbName == "" {
		klog.Errorf("Required argument is missing")
		return errors.New("Required argument is missing")
	}
	c, err := GetCloudProviderVpc()
	if err != nil {
		klog.Errorf("GetCloudProviderVpc failed: %v\n", err)
		return err
	}

	// Retrieve the VPC load balancer matching the name/id that passed in
	lb, err := c.FindLoadBalancer(lbName, nil)
	if err != nil {
		klog.Errorf("FindLoadBalancer failed: %v\n", err)
		return err
	}
	if lb == nil {
		// The specified VPC load balancer was not found.  Return the "NOT_FOUND" message to the cloud provider
		klog.NotFound("Load balancer %s not found", lbName)
		return nil
	}

	// Return basic stats about the load balancer for the cloud provider to log
	klog.Infof(lb.GetSummary())

	// If the VPC load balancer is Ready, return success
	if lb.IsReady() {
		klog.Success(lb.GetSuccessString())
		return nil
	}

	// Return the pending status since there are operations in progress
	klog.Pending(lb.GetStatus())
	return nil
}

// UpdateLoadBalancer - method gets called by the cloud provider UpdateLoadBalancer method.
//
// This method will get called when:
//  - nodes are added or removed from the cluster
func UpdateLoadBalancer(lbName, kubeService string) error {
	klog.Infof("Entering UpdateLoadBalancer(%s, %s)", lbName, kubeService)
	if lbName == "" {
		klog.Errorf("Required argument is missing")
		return errors.New("Required argument is missing")
	}

	// Initialize Cloud VPC
	c, err := GetCloudProviderVpc()
	if err != nil {
		klog.Errorf("GetCloudProviderVpc failed: %v\n", err)
		return err
	}

	// If only 1 arg was passed in, then assume it is the kubeService name and not the lbName
	if kubeService == "" {
		kubeService = lbName
		lbName = ""
	}
	// Locate the correct Kubernetes service
	service, err := getKubeService(c.KubeClient, kubeService)
	if err != nil {
		klog.Errorf("getKubeService failed: %v\n", err)
		return err
	}
	// If we don't have a name for the VPC load balancer, generate one
	if lbName == "" {
		lbName = c.GenerateLoadBalancerName(service)
	}

	// Retrieve list of nodes
	nodes, err := getKubeNodes(c.KubeClient)
	if err != nil {
		klog.Errorf("getNodes failed: %v\n", err)
		return err
	}

	// Check to see if the VPC load balancer already exists
	lb, err := c.FindLoadBalancer(lbName, service)
	if err != nil {
		klog.Errorf("FindLoadBalancer failed: %v\n", err)
		return err
	}
	if lb == nil {
		// The specified VPC load balancer was not found.  Return an error
		klog.Errorf("Load balancer %s not found", lbName)
		return fmt.Errorf("Load balancer %s not found", lbName)
	}

	// Return basic stats about the load balancer for the cloud provider to log
	klog.Infof(lb.GetSummary())

	// Check the state of the load balancer to determine if the update operation can even be attempted
	if !lb.IsReady() {
		klog.Pending(lb.GetStatus())
		return nil
	}

	// Update the existing LB with any service or node changes that may have occurred
	lb, err = c.UpdateLoadBalancer(lb, service, nodes)
	if err != nil {
		klog.Errorf("UpdateLoadBalancer failed: %v\n", err)
		return err
	}
	klog.Success(lb.GetSuccessString())
	return nil
}
