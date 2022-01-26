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

package ibm

import (
	"context"
	"fmt"
	"os"
	"strings"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/klog"
	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpcctl"
	v1 "k8s.io/api/core/v1"
)

const (
	vpcLbStatusOnlineActive              = vpcctl.LoadBalancerOperatingStatusOnline + "/" + vpcctl.LoadBalancerProvisioningStatusActive
	vpcLbStatusOfflineCreatePending      = vpcctl.LoadBalancerOperatingStatusOffline + "/" + vpcctl.LoadBalancerProvisioningStatusCreatePending
	vpcLbStatusOfflineMaintenancePending = vpcctl.LoadBalancerOperatingStatusOffline + "/" + vpcctl.LoadBalancerProvisioningStatusMaintenancePending
	vpcLbStatusOfflineFailed             = vpcctl.LoadBalancerOperatingStatusOffline + "/" + vpcctl.LoadBalancerProvisioningStatusFailed
	vpcLbStatusOfflineNotFound           = vpcctl.LoadBalancerOperatingStatusOffline + "/not_found"
)

// InitCloudVpc - Initialize the VPC cloud logic
func (c *Cloud) InitCloudVpc(enablePrivateEndpoint bool) (*vpcctl.CloudVpc, error) {
	// Extract the VPC cloud object. If set, return it
	cloudVpc := c.Vpc
	if cloudVpc != nil {
		return cloudVpc, nil
	}
	// Initialize config based on values in the cloud provider
	config, err := c.NewConfigVpc(enablePrivateEndpoint)
	if err != nil {
		return nil, err
	}
	// Allocate a new VPC Cloud object and save it if successful
	cloudVpc, err = vpcctl.NewCloudVpc(c.KubeClient, config)
	if cloudVpc != nil {
		c.Vpc = cloudVpc
	}
	return cloudVpc, err
}

// isProviderVpc - Is the current cloud provider running in VPC environment?
func (c *Cloud) isProviderVpc() bool {
	provider := c.Config.Prov.ProviderType
	return provider == vpcctl.VpcProviderTypeGen2
}

// NewConfigVpc - Create the ConfigVpc from the current Cloud object
func (c *Cloud) NewConfigVpc(enablePrivateEndpoint bool) (*vpcctl.ConfigVpc, error) {
	// Make sure Cloud config has been initialized
	if c.Config == nil {
		return nil, fmt.Errorf("Cloud config not initialized")
	}
	// Initialize config based on values in the cloud provider
	config := &vpcctl.ConfigVpc{
		AccountID:         c.Config.Prov.AccountID,
		ClusterID:         c.Config.Prov.ClusterID,
		EnablePrivate:     enablePrivateEndpoint,
		ProviderType:      c.Config.Prov.ProviderType,
		Region:            c.Config.Prov.Region,
		ResourceGroupName: c.Config.Prov.G2ResourceGroupName,
		SubnetNames:       c.Config.Prov.G2VpcSubnetNames,
		WorkerAccountID:   c.Config.Prov.G2WorkerServiceAccountID,
		VpcName:           c.Config.Prov.G2VpcName,
	}
	// If the G2Credentials is set, then look up the API key
	if c.Config.Prov.G2Credentials != "" {
		apiKey, err := os.ReadFile(c.Config.Prov.G2Credentials)
		if err != nil {
			return nil, fmt.Errorf("Failed to read credentials from %s: %v", c.Config.Prov.G2Credentials, err)
		}
		config.APIKeySecret = strings.TrimSpace(string(apiKey))
	}
	return config, nil
}

// VpcEnsureLoadBalancer - Creates a new VPC load balancer or updates the existing one. Returns the status of the balancer
func (c *Cloud) VpcEnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	lbName := c.vpcGetLoadBalancerName(service)
	klog.Infof("EnsureLoadBalancer(lbName:%v, Service:{%v}, NodeCount:%v)", lbName, c.vpcGetServiceDetails(service), len(nodes))
	if len(nodes) == 0 {
		errString := "There are no available nodes for LoadBalancer"
		klog.Errorf(errString)
		return nil, fmt.Errorf(errString)
	}
	vpc, err := c.InitCloudVpc(true)
	if err != nil {
		errString := fmt.Sprintf("Failed initializing VPC: %v", err)
		klog.Errorf(errString)
		return nil, c.Recorder.VpcLoadBalancerServiceWarningEvent(service, CreatingCloudLoadBalancerFailed, lbName, errString)
	}
	// Attempt to create/update the VPC load balancer for this service
	status, err := vpc.VpcEnsureLoadBalancer(lbName, service, nodes)
	if err != nil {
		return nil, c.Recorder.VpcLoadBalancerServiceWarningEvent(service, CreatingCloudLoadBalancerFailed, lbName, err.Error())
	}
	return status, nil
}

// VpcEnsureLoadBalancerDeleted - Deletes the specified load balancer if it exists,
// returning nil if the load balancer specified either didn't exist or was successfully deleted.
func (c *Cloud) VpcEnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lbName := c.vpcGetLoadBalancerName(service)
	klog.Infof("EnsureLoadBalancerDeleted(lbName:%v, Service:{%v})", lbName, c.vpcGetServiceDetails(service))
	vpc, err := c.InitCloudVpc(true)
	if err != nil {
		errString := fmt.Sprintf("Failed initializing VPC: %v", err)
		klog.Errorf(errString)
		return c.Recorder.VpcLoadBalancerServiceWarningEvent(service, DeletingCloudLoadBalancerFailed, lbName, errString)
	}
	// Attempt to delete the VPC load balancer
	err = vpc.VpcEnsureLoadBalancerDeleted(lbName, service)
	if err != nil {
		return c.Recorder.VpcLoadBalancerServiceWarningEvent(service, DeletingCloudLoadBalancerFailed, lbName, err.Error())
	}
	return nil
}

// vpcGetEventMessage based on the status that was passed in
func (c *Cloud) vpcGetEventMessage(status string) string {
	switch status {
	case vpcLbStatusOfflineFailed:
		return "The VPC load balancer that routes requests to this Kubernetes LoadBalancer service is offline. For troubleshooting steps, see <https://ibm.biz/vpc-lb-ts>"
	case vpcLbStatusOfflineMaintenancePending:
		return "The VPC load balancer that routes requests to this Kubernetes LoadBalancer service is under maintenance."
	case vpcLbStatusOfflineNotFound:
		return "The VPC load balancer that routes requests to this Kubernetes LoadBalancer service was not found. To recreate the VPC load balancer, restart the Kubernetes master by running 'ibmcloud ks cluster master refresh --cluster <cluster_name_or_id>'."
	default:
		return fmt.Sprintf("The VPC load balancer that routes requests to this Kubernetes LoadBalancer service is currently %s.", status)
	}
}

// VpcGetLoadBalancer - Returns whether the specified load balancer exists, and
// if so, what its status is.
func (c *Cloud) VpcGetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	lbName := c.vpcGetLoadBalancerName(service)
	klog.Infof("GetLoadBalancer(lbName:%v, Service:{%v})", lbName, c.vpcGetServiceDetails(service))
	vpc, err := c.InitCloudVpc(true)
	if err != nil {
		errString := fmt.Sprintf("Failed initializing VPC: %v", err)
		klog.Errorf(errString)
		return nil, false, c.Recorder.VpcLoadBalancerServiceWarningEvent(service, GettingCloudLoadBalancerFailed, lbName, errString)
	}
	// Retrieve the status of the VPC load balancer
	status, exist, err := vpc.VpcGetLoadBalancer(lbName, service)
	if err != nil {
		return status, exist, c.Recorder.VpcLoadBalancerServiceWarningEvent(service, GettingCloudLoadBalancerFailed, lbName, err.Error())
	}
	return status, exist, nil
}

// vpcGetLoadBalancerName - Returns the name of the load balancer
func (c *Cloud) vpcGetLoadBalancerName(service *v1.Service) string {
	clusterID := c.Config.Prov.ClusterID
	serviceID := strings.ReplaceAll(string(service.UID), "-", "")
	ret := vpcctl.VpcLbNamePrefix + "-" + clusterID + "-" + serviceID
	// Limit the LB name to 63 characters
	if len(ret) > 63 {
		ret = ret[:63]
	}
	return ret
}

// vpcGetServiceDetails - Returns the string of the Kube LB service key fields
func (c *Cloud) vpcGetServiceDetails(service *v1.Service) string {
	if service == nil {
		return "<nil>"
	}
	// Only include the service annotations that we care about in the log
	annotations := map[string]string{}
	for k, v := range service.ObjectMeta.Annotations {
		if strings.Contains(k, "ibm-load-balancer-cloud-provider") {
			annotations[k] = v
		}
	}
	// Only include the port information that we care about: protocol, ext port, node port
	ports := []string{}
	for _, port := range service.Spec.Ports {
		portString := fmt.Sprintf("%v-%v-%v", port.Protocol, port.Port, port.NodePort)
		ports = append(ports, strings.ToLower(portString))
	}
	return fmt.Sprintf("Name:%v NameSpace:%v UID:%v Annotations:%v Ports:%v ExternalTrafficPolicy:%v HealthCheckNodePort:%v Status:%+v",
		service.ObjectMeta.Name,
		service.ObjectMeta.Namespace,
		service.ObjectMeta.UID,
		annotations,
		ports,
		service.Spec.ExternalTrafficPolicy,
		service.Spec.HealthCheckNodePort,
		service.Status)
}

// VpcHandleSecret is called to process the add/delete/update of a Kubernetes secret
func (c *Cloud) VpcHandleSecret(secret *v1.Secret, action string) {
	// If the VPC environment has not been initialzed, simply return
	vpc := c.Vpc
	if vpc == nil {
		return
	}
	// Check the secret to determine if VPC settings need to be reset
	if vpc.IsVpcConfigStoredInSecret(secret) {
		klog.Infof("VCP secret %s/%s had been %s. Reset the VPC config data", secret.ObjectMeta.Namespace, secret.ObjectMeta.Name, action)
		c.Vpc = nil
	}
}

// VpcHandleSecretAdd is called when a secret is added
func (c *Cloud) VpcHandleSecretAdd(obj interface{}) {
	secret := obj.(*v1.Secret)
	c.VpcHandleSecret(secret, "added")
}

// VpcHandleSecretDelete is called when a secret is deleted
func (c *Cloud) VpcHandleSecretDelete(obj interface{}) {
	secret := obj.(*v1.Secret)
	c.VpcHandleSecret(secret, "deleted")
}

// VpcHandleSecretUpdate is called when a secret is changed
func (c *Cloud) VpcHandleSecretUpdate(oldObj, newObj interface{}) {
	secret := newObj.(*v1.Secret)
	c.VpcHandleSecret(secret, "updated")
}

// VpcMonitorLoadBalancers accepts a list of services (of all types), verifies that each Kubernetes load balancer service has a
// corresponding VPC load balancer object, and creates Kubernetes events based on the load balancer's status.
// `status` is a map from a load balancer's unique Service ID to its status.
// This persists load balancer status between consecutive monitor calls.
func (c *Cloud) VpcMonitorLoadBalancers(services *v1.ServiceList, status map[string]string) {
	// If there are no load balancer services to monitor, don't even initCloudVpc, just return.
	if services == nil || len(services.Items) == 0 {
		klog.Infof("No Load Balancers to monitor, returning")
		return
	}

	vpc, err := c.InitCloudVpc(true)
	if err != nil {
		klog.Errorf("Failed initializing VPC: %v", err)
		return
	}
	// Retrieve VPC LBs for the current cluster
	lbMap, vpcMap, err := vpc.VpcMonitorLoadBalancers(services)
	if err != nil {
		klog.Errorf("Failed retrieving VPC LBs: %v", err)
		return
	}

	// Verify that we have a VPC LB for each of the Kube LB services
	for lbName, service := range lbMap {
		serviceID := string(service.ObjectMeta.UID)
		oldStatus := status[serviceID]
		vpcLB, exists := vpcMap[lbName]
		if exists {
			// Store the new status so its available to the next call to VpcMonitorLoadBalancers()
			newStatus := vpcLB.GetStatus()
			status[serviceID] = newStatus

			// If the current state of the LB is online/active
			if newStatus == vpcLbStatusOnlineActive {
				if oldStatus != vpcLbStatusOnlineActive {
					// If the status of the VPC load balancer transitioned to 'online/active' --> NORMAL EVENT.
					c.Recorder.VpcLoadBalancerServiceNormalEvent(service, CloudVPCLoadBalancerNormalEvent, lbName, c.vpcGetEventMessage(newStatus))
				}
				// Move on to the next LB service
				continue
			}

			// If the status of the VPC load balancer is not 'online/active', record warning event if status has not changed since last we checked
			klog.Infof("VPC load balance not online/active: ServiceID:%s Service:%s/%s %s",
				string(service.ObjectMeta.UID), service.ObjectMeta.Namespace, service.ObjectMeta.Name, vpcLB.GetSummary())
			if oldStatus == newStatus {
				_ = c.Recorder.VpcLoadBalancerServiceWarningEvent(
					service, VerifyingCloudLoadBalancerFailed, lbName, c.vpcGetEventMessage(newStatus)) // #nosec G104 error is always returned
			}

			// Move on to the next LB service
			continue
		}

		// There is no VPC LB for the current Kubernetes load balancer.  Update the status to: "offline/not_found"
		klog.Warningf("VPC load balancer not found for service %s %s/%s ", serviceID, service.ObjectMeta.Namespace, service.ObjectMeta.Name)
		newStatus := vpcLbStatusOfflineNotFound
		status[serviceID] = newStatus
		if oldStatus == newStatus {
			_ = c.Recorder.VpcLoadBalancerServiceWarningEvent(
				service, VerifyingCloudLoadBalancerFailed, lbName, c.vpcGetEventMessage(newStatus)) // #nosec G104 error is always returned
		}
	}
}

// VpcUpdateLoadBalancer updates hosts under the specified load balancer
func (c *Cloud) VpcUpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	lbName := c.vpcGetLoadBalancerName(service)
	klog.Infof("UpdateLoadBalancer(lbName:%v, Service:{%v}, NodeCount:%v)", lbName, c.vpcGetServiceDetails(service), len(nodes))
	if len(nodes) == 0 {
		errString := "There are no available nodes for LoadBalancer"
		klog.Errorf(errString)
		return fmt.Errorf(errString)
	}
	vpc, err := c.InitCloudVpc(true)
	if err != nil {
		errString := fmt.Sprintf("Failed initializing VPC: %v", err)
		klog.Errorf(errString)
		return c.Recorder.VpcLoadBalancerServiceWarningEvent(service, UpdatingCloudLoadBalancerFailed, lbName, errString)
	}
	// Update the VPC load balancer
	err = vpc.VpcUpdateLoadBalancer(lbName, service, nodes)
	if err != nil {
		if strings.Contains(err.Error(), "Load balancer not found") {
			return nil
		}
		return c.Recorder.VpcLoadBalancerServiceWarningEvent(service, UpdatingCloudLoadBalancerFailed, lbName, err.Error())
	}
	return nil
}
