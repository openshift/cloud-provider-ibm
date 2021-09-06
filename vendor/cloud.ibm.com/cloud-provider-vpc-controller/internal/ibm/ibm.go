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
	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpclb"
	clientset "k8s.io/client-go/kubernetes"
)

// Provider holds information from the cloud provider node (i.e. instance).
type Provider struct {
	// Optional: Cluster ID of the master. Only set in controller manager.
	ClusterID string
}

// CloudConfig is the ibm cloud provider config data.
type CloudConfig struct {
	// [provider] section
	Prov Provider
}

// Cloud is the ibm cloud provider implementation.
type Cloud struct {
	KubeClient clientset.Interface
	Config     *CloudConfig
	Vpc        *vpclb.CloudVpc
}
