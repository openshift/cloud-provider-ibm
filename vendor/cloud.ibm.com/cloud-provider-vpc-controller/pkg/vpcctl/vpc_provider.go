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

package vpcctl

import (
	"errors"
	"fmt"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/klog"
	v1 "k8s.io/api/core/v1"
)

// VpcEnsureLoadBalancer - called by cloud provider to create/update the load balancer
func (c *CloudVpc) VpcEnsureLoadBalancer(lbName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	// Check to see if the VPC load balancer exists
	lb, err := c.FindLoadBalancer(lbName, service)
	if err != nil {
		errString := fmt.Sprintf("Failed getting LoadBalancer: %v", err)
		klog.Errorf(errString)
		return nil, fmt.Errorf(errString)
	}

	// If the specified VPC load balancer was not found, create it
	if lb == nil {
		lb, err = c.CreateLoadBalancer(lbName, service, nodes)
		if err != nil {
			errString := fmt.Sprintf("Failed ensuring LoadBalancer: %v", err)
			klog.Errorf(errString)
			return nil, fmt.Errorf(errString)
		}
		// Log basic stats about the load balancer and return success (if the LB is READY or not NLB)
		// - return SUCCESS for non-NLB to remain backward compatibility, no additional operations need to be done
		// - don't return SUCCESS for NLB, because we can't do the DNS lookup of static IP if the LB is still pending
		if lb.IsReady() || !lb.IsNLB() {
			klog.Infof(lb.GetSummary())
			klog.Infof("Load balancer %v created.  Hostname: %v", lbName, lb.Hostname)
			return c.GetLoadBalancerStatus(service, lb.Hostname), nil
		}
	}

	// Log basic stats about the load balancer
	klog.Infof(lb.GetSummary())

	// If we get to this point, it means that EnsureLoadBalancer was called against a Load Balancer
	// that already exists. This is most likely due to a change is the Kubernetes service.
	// If the load balancer is not "Online/Active", then no additional operations that can be performed.
	if !lb.IsReady() {
		klog.Warningf("Load balancer %v is busy: %v", lbName, lb.GetStatus())
		return nil, fmt.Errorf("LoadBalancer is busy: %v", lb.GetStatus())
	}

	// The load balancer state is Online/Active.  This means that additional operations can be done.
	// Update the existing LB with any service or node changes that may have occurred.
	lb, err = c.UpdateLoadBalancer(lb, service, nodes)
	if err != nil {
		errString := fmt.Sprintf("Failed ensuring LoadBalancer: %v", err)
		klog.Errorf(errString)
		return nil, fmt.Errorf(errString)
	}

	// Return success
	klog.Infof("Load balancer %v created.  Hostname: %v", lbName, lb.Hostname)
	return c.GetLoadBalancerStatus(service, lb.Hostname), nil
}

// VpcEnsureLoadBalancerDeleted - called by cloud provider to delete the load balancer
func (c *CloudVpc) VpcEnsureLoadBalancerDeleted(lbName string, service *v1.Service) error {
	// Check to see if the VPC load balancer exists
	lb, err := c.FindLoadBalancer(lbName, service)
	if err != nil {
		errString := fmt.Sprintf("Failed getting LoadBalancer: %v", err)
		klog.Errorf(errString)
		return fmt.Errorf(errString)
	}

	// If the load balancer does not exist, return
	if lb == nil {
		klog.Infof("Load balancer %v not found", lbName)
		return nil
	}

	// Log basic stats about the load balancer
	klog.Infof(lb.GetSummary())

	// The load balancer state is Online/Active.  Attempt to delete the load balancer
	err = c.DeleteLoadBalancer(lb, service)
	if err != nil {
		errString := fmt.Sprintf("Failed deleting LoadBalancer: %v", err)
		klog.Errorf(errString)
		return fmt.Errorf(errString)
	}

	// Return success
	klog.Infof("Load balancer %v deleted", lbName)
	return nil
}

// VpcGetLoadBalancer - called by cloud provider to retrieve status of the load balancer
func (c *CloudVpc) VpcGetLoadBalancer(lbName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	// Check to see if the VPC load balancer exists
	lb, err := c.FindLoadBalancer(lbName, service)
	if err != nil {
		errString := fmt.Sprintf("Failed getting LoadBalancer: %v", err)
		klog.Errorf(errString)
		return nil, false, fmt.Errorf(errString)
	}

	// The load balancer was not found
	if lb == nil {
		klog.Infof("Load balancer %v not found", lbName)
		return nil, false, nil
	}

	// Write details of the load balancer to the log
	klog.Infof(lb.GetSummary())

	// If the VPC load balancer is not Ready, return the hostname from the service or blank
	if !lb.IsReady() {
		klog.Warningf("Load balancer %s is busy: %v", lbName, lb.GetStatus())
		var lbStatus *v1.LoadBalancerStatus
		if service.Status.LoadBalancer.Ingress != nil {
			lbStatus = c.GetLoadBalancerStatus(service, lb.Hostname)
		} else {
			lbStatus = &v1.LoadBalancerStatus{}
		}
		return lbStatus, true, nil
	}

	// Return success
	klog.Infof("Load balancer %v found.  Hostname: %v", lbName, lb.Hostname)
	return c.GetLoadBalancerStatus(service, lb.Hostname), true, nil
}

// VpcMonitorLoadBalancers - returns status of all VPC load balancers associated with Kube LBs in this cluster
func (c *CloudVpc) VpcMonitorLoadBalancers(services *v1.ServiceList) (map[string]*v1.Service, map[string]*VpcLoadBalancer, error) {
	// Verify we were passed a list of Kube services
	if services == nil {
		klog.Errorf("Required argument is missing")
		return nil, nil, errors.New("Required argument is missing")
	}
	// Retrieve load balancers for the current cluster
	vpcMap, err := c.GatherLoadBalancers(services)
	if err != nil {
		return nil, nil, err
	}
	// Create map of Kube node port and LB services
	lbMap := map[string]*v1.Service{}
	npMap := map[string]*v1.Service{}
	for _, service := range services.Items {
		// Keep track of all load balancer -AND- node port services.
		//
		// The cloud provider will only ever create VPC LB for a Load Balancer service,
		// but the vpcctl binary can create a VPC LB for a Node Port service.  This allows testing
		// of create/delete VPC LB functionality outside of the cloud provider.
		//
		// This means that it is possible to have VPC LB point to either a Kube LB or Kube node port
		kubeService := service
		switch kubeService.Spec.Type {
		case v1.ServiceTypeLoadBalancer:
			lbName := c.GenerateLoadBalancerName(&kubeService)
			lbMap[lbName] = &kubeService
		case v1.ServiceTypeNodePort:
			lbName := c.GenerateLoadBalancerName(&kubeService)
			npMap[lbName] = &kubeService
		}
	}

	// Clean up any VPC LBs that are in READY state that do not have Kube LB or node port service
	for _, lb := range vpcMap {
		if !lb.IsReady() {
			continue
		}
		// If we have a VPC LB and there is no Kube LB or node port service associated with it,
		// go ahead and schedule the deletion of the VPC LB. The fact that we are deleting the
		// VPC LB will be displayed as an "INFO:"" statement in the vpcctl stdout and will be added
		// to the cloud provider controller manager log. Since there is no "ServiceUID:" on this
		// "INFO:" statement, it will just be logged.
		if lbMap[lb.Name] == nil && npMap[lb.Name] == nil {
			klog.Infof("Deleting stale VPC LB: %s", lb.GetSummary())
			err := c.DeleteLoadBalancer(lb, nil)
			if err != nil {
				// Add an error message to log, but don't fail the entire MONITOR operation
				klog.Errorf("Failed to delete stale VPC LB: %s", lb.Name)
			}
		}
	}

	// Return the LB and VPC maps to the caller
	return lbMap, vpcMap, nil
}

// VpcUpdateLoadBalancer - updates the hosts under the specified load balancer
func (c *CloudVpc) VpcUpdateLoadBalancer(lbName string, service *v1.Service, nodes []*v1.Node) error {
	// Check to see if the VPC load balancer exists
	lb, err := c.FindLoadBalancer(lbName, service)
	if err != nil {
		errString := fmt.Sprintf("Failed getting LoadBalancer: %v", err)
		klog.Errorf(errString)
		return fmt.Errorf(errString)
	}
	if lb == nil {
		errString := fmt.Sprintf("Load balancer not found: %v", lbName)
		klog.Errorf(errString)
		return fmt.Errorf(errString)
	}
	// Log basic stats about the load balancer
	klog.Infof(lb.GetSummary())

	// Check the state of the load balancer to determine if the update operation can even be attempted
	if !lb.IsReady() {
		klog.Warningf("Load balancer %v is busy: %v", lbName, lb.GetStatus())
		return fmt.Errorf("LoadBalancer is busy: %v", lb.GetStatus())
	}

	// The load balancer state is Online/Active.  This means that additional operations can be done.
	// Update the existing LB with any service or node changes that may have occurred.
	_, err = c.UpdateLoadBalancer(lb, service, nodes)
	if err != nil {
		errString := fmt.Sprintf("Failed updating LoadBalancer: %v", err)
		klog.Errorf(errString)
		return fmt.Errorf(errString)
	}

	// Return success
	klog.Infof("Load balancer %v updated.", lbName)
	return nil
}
