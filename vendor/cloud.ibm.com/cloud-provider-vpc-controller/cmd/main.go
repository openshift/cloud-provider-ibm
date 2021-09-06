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

// Program vpcctl allows VPC load balancers to be configured
package main

import (
	"fmt"
	"os"
	"strings"

	"cloud.ibm.com/cloud-provider-vpc-controller/internal/cloud"
)

// Global variables
var (
	version string
)

// Usage information
func showHelp() {
	fmt.Printf("\nValid commands [optional] <required>:\n\n")
	fmt.Printf("   %-54v - %v\n", "create-lb           [LB name] <Kube service>", "Creates a new load balancer or updates the existing one")
	fmt.Printf("   %-54v - %v\n", "delete-lb           <LB name | ID | Kube service>", "Delete the specified load balancer if it exists")
	fmt.Printf("   %-54v - %v\n", "monitor", "Verify that a load balancer exists for each Kube LB service")
	fmt.Printf("   %-54v - %v\n", "status-lb           <LB name | ID>", "Returns the status of the specified load balancer, if it exists")
	fmt.Printf("   %-54v - %v\n", "update-lb           [LB name | ID] <Kube service>", "Update the load balancer associated with the specified service")
	fmt.Printf("\n")
	fmt.Printf("   %-54v - %v\n", "help", "Display this help message")
	fmt.Printf("   %-54v - %v\n", "version", "Display current version")
	fmt.Printf("\n")
}

// main ...
func main() {
	if len(os.Args) < 2 {
		showHelp()
		return
	}
	cmd := strings.ToLower(os.Args[1])
	var arg1 string
	var arg2 string
	if len(os.Args) > 2 {
		arg1 = os.Args[2]
	}
	if len(os.Args) > 3 {
		arg2 = os.Args[3]
	}
	_ = runCmd(cmd, arg1, arg2)
}

// Run the requested command with the specified args
func runCmd(cmd, arg1, arg2 string) error {
	var err error
	switch cmd {
	case "create-lb":
		err = cloud.CreateLoadBalancer(arg1, arg2)
	case "delete-lb":
		err = cloud.DeleteLoadBalancer(arg1)
	case "monitor":
		err = cloud.MonitorLoadBalancers()
	case "status-lb":
		err = cloud.StatusLoadBalancer(arg1)
	case "update-lb":
		err = cloud.UpdateLoadBalancer(arg1, arg2)

	// Need to include "sdk-create-lb" because it is used by cloud provider for "proxy-protocol"
	case "sdk-create-lb":
		err = cloud.CreateLoadBalancer(arg1, arg2)

	// Miscellaneous options
	case "help":
		showHelp()
	case "version":
		fmt.Printf("Version: %s\n", version)
	default:
		fmt.Printf("Invalid option: %s\n", cmd)
		showHelp()
		return fmt.Errorf("Invalid option: %s", cmd)
	}

	return err
}
