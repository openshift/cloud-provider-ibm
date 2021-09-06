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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/klog"
	"cloud.ibm.com/cloud-provider-vpc-controller/pkg/vpclb"
	"gopkg.in/gcfg.v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // needed for cloud-provider
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	clusterNamespace     = "kube-system"
	clusterConfigMap     = "cluster-info"
	clusterConfigKey     = "cluster-config.json"
	envVarIbmCloudConfig = "VPCCTL_CLOUD_CONFIG"
	envVarKubeConfig     = "KUBECONFIG"
	envVarLbPrefix       = "VPCCTL_LB_PREFIX"
	envVarPublicEndPoint = "VPCCTL_PUBLIC_ENDPOINT"
	ibmCloudConfigINI    = "/mnt/etc/kubernetes/ibm-cloud-config.ini"
)

type clusterInfo struct {
	ClusterID string `json:"cluster_id"`
}

type IbmCloudConfig struct {
	Provider struct {
		ClusterID string `gcfg:"ClusterID"`
	}
}

// Variable that can be overridden by unit tests
var getKubernetesClient = getKubectl
var runtimeOS = runtime.GOOS

// Single init routine to perform some common initialization
func cloudInit() (kubernetes.Interface, string, error) {
	client, err := getKubernetesClient()
	if err != nil {
		return nil, "", err
	}
	clusterID, err := getClusterID(client)
	if err != nil {
		return client, "", err
	}
	// If env var is set, use the specified VPC LB prefix instead of "kube-"
	lbPrefix := strings.ToLower(os.Getenv(envVarLbPrefix))
	if lbPrefix != "" {
		vpclb.VpcLbNamePrefix = lbPrefix
	}
	return client, clusterID, err
}

// getClusterIDFromINIFile - Retrieve the IKS cluster id from INI file
func getClusterIDFromINIFile(f string) (string, error) {
	if _, err := os.Stat(f); os.IsNotExist(err) {
		klog.Infof("Missing INI file: %s\n", f)
		return "", fmt.Errorf("missing INI file: %s", f)
	}
	var cfg IbmCloudConfig
	err := gcfg.FatalOnly(gcfg.ReadFileInto(&cfg, f))
	if err != nil {
		klog.Infof("Fatal error during INI processing: %s\n", f)
		return "", fmt.Errorf("fatal error occurs during the INI file processing")
	}
	if cfg.Provider.ClusterID == "" {
		klog.Infof("Missing or empty ClusterID in INI file: %s\n", f)
		return "", fmt.Errorf("missing or empty ClusterID in INI file: %s", f)
	}
	return cfg.Provider.ClusterID, nil
}

// getClusterID - Retrieve the IKS cluster id
func getClusterID(client kubernetes.Interface) (string, error) {
	ibmcc, errb := os.LookupEnv(envVarIbmCloudConfig)
	// If the environment variable is not exist or empty then use the default INI file
	if (!errb) || (ibmcc == "") {
		ibmcc = ibmCloudConfigINI
	} else {
		klog.Infof("%s environment variable is non-empty (value: %s), so using that file\n", envVarIbmCloudConfig, ibmcc)
	}
	cid, err := getClusterIDFromINIFile(ibmcc)
	// If we could get the cluster id from the INI file then return it otherwise get it from config map
	if err == nil {
		klog.Infof("Processed the %s file and found the following clusterid: %s\n", ibmcc, cid)
		return cid, nil
	}
	cm, err := client.CoreV1().ConfigMaps(clusterNamespace).Get(context.TODO(), clusterConfigMap, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get %s/%s config map: %v", clusterNamespace, clusterConfigMap, err)
	}
	configData := cm.Data[clusterConfigKey]
	if configData == "" {
		return "", fmt.Errorf("%s/%s config map does not contain key: [%s]", clusterNamespace, clusterConfigMap, clusterConfigKey)
	}
	jsonData := &clusterInfo{}
	err = json.Unmarshal([]byte(configData), jsonData)
	if err != nil {
		return "", fmt.Errorf("failed to un-marshall config data: [%s]: %v", configData, err)
	}
	return jsonData.ClusterID, nil
}

// getKubectl - Create client connection to kubernetes master
func getKubectl() (kubernetes.Interface, error) {
	var config *rest.Config
	var err error

	// Create the client config
	kubeConfig := os.Getenv(envVarKubeConfig)
	if kubeConfig == "" {
		homeDir, _ := os.UserHomeDir()
		defaultKubeConfig := filepath.Join(homeDir, ".kube", "config")
		if _, err = os.Stat(defaultKubeConfig); err == nil {
			kubeConfig = defaultKubeConfig
		}
	}
	if kubeConfig == "" {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	}
	if err != nil {
		return nil, err
	}

	// Create the client
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// getKubeNodes - get all of the nodes in the cluster
func getKubeNodes(client kubernetes.Interface) ([]*v1.Node, error) {
	nodelist, err := client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get list of nodes: %v", err)
	}
	nodes := []*v1.Node{}
	for _, node := range nodelist.Items {
		// If the node has been cordoned, don't use it as a pool member
		if node.Spec.Unschedulable {
			continue
		}
		// If the node is not "Ready", don't use it as a pool member
		nodeReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == v1.NodeReady {
				nodeReady = (condition.Status == v1.ConditionTrue)
				break
			}
		}
		if !nodeReady {
			continue
		}
		kubeNode := node
		nodes = append(nodes, &kubeNode)
	}
	return nodes, nil
}

// getKubeService - Retrieve the Kubernetes service
func getKubeService(client kubernetes.Interface, kubeService string) (*v1.Service, error) {
	namespace := "default"
	name := kubeService
	if strings.Contains(kubeService, "/") {
		namespace = strings.Split(kubeService, "/")[0]
		name = strings.Split(kubeService, "/")[1]
	}
	service, err := client.CoreV1().Services(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get service %s/%s: %v", namespace, name, err)
	}
	return service, nil
}

// getKubeServices - Retrieve list of all Kubernetes services
func getKubeServices(client kubernetes.Interface) (*v1.ServiceList, error) {
	serviceList, err := client.CoreV1().Services(v1.NamespaceAll).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to get list of Kubernetes services: %v", err)
	}
	return serviceList, nil
}

// shouldPrivateEndpointBeEnabled - Determine if private service endpoint should be enabled
func shouldPrivateEndpointBeEnabled() bool {
	// If we are not running on "Linux", always return "false"
	if runtimeOS != "linux" {
		return false
	}
	// If "public-only" environment variable is set, return "false"
	if strings.ToLower(os.Getenv(envVarPublicEndPoint)) == "true" {
		return false
	}
	// Default to "true" on Linux build
	return true
}
