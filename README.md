# IBM Cloud Provider

This is the IBM Cloud Provider repository which implements the
IBM Cloud Controller Manager (CCM). The IBM CCM can be used to provide IBM Cloud
infrastructure node and load balancer support to
[Kubernetes](https://kubernetes.io/docs/home/) or
[OpenShift](https://docs.openshift.com/) clusters running on
[IBM Cloud](https://cloud.ibm.com/docs). This repository branch is based on
[Kubernetes version v1.33.1](https://github.com/kubernetes/kubernetes/tree/v1.33.1).

See [CONTRIBUTING.md](./CONTRIBUTING.md) for contribution guidelines.

## Local Build and Deploy Instructions

### Building IBM Cloud Provider from your Local Repository

These build instructions have been verified using
[VirtualBox version 6.1.36](https://www.virtualbox.org/wiki/Downloads),
[Vagrant version 2.2.19](https://www.vagrantup.com/downloads), and
MacOS version 12.5.

1. Create VirtualBox network configuration `/etc/vbox/network.conf` with the following content:

   ```bash
   $ cat /etc/vbox/network.conf
   * 192.168.10.0/21 192.168.56.0/21
   ```

1. Change to your local repository. The build will work against this directory
   by making it a vagrant folder synchronized to the VM.

1. `cd vagrant-kube-build`

1. `./build.sh`

1. If the build fails, you can correct the errors and re-run `./build.sh`.
   You can also run specific build steps by passing one or more of the following
   options:

   * Build setup (always run with other steps to setup the build environment and dependencies): `setup`
   * Build source (i.e. `make fmt`, `make lint`, etc.): `source`
   * Build containers (i.e. `make containers`): `containers`
   * Build Docker registry for the containers: `registry`
   * Build specific make step (cannot be run with other steps): `make [[option] ...]`

1. Once the build is complete, you can log into the VM via `vagrant ssh`.
   Running `vagrant ssh-config` will give you SSH configuration updates which
   you can apply to your host's `~/.ssh/config` file. This allows you to run
   `ssh cloud-provider-ibm-build` to access the VM. Also, you can use
   `vagrant suspend` to suspend the VM and `vagrant destroy -f` to destroy it.

### Deploying IBM Cloud Provider

Refer to [examples](./docs/examples) for deploying the IBM Cloud Provider based
on underlying infrastructure type, classic or VPC.

## Testing

### Unit

The default build process above will run all unit tests and matches
what is done by Travis CI.

`make test`

## Dependencies

Dependencies are managed via [go modules](https://github.com/golang/go/wiki/Modules)
for builds. Be sure to regenerate the `go.mod` and `go.sum` files when there are
new or updated dependencies. You can do this by running `make updatedeps`.

## Kubernetes Version Update Process

The steps needed to support a new Kubernetes release have been moved to a Wiki page.

## Release Process

Travis CI is used to build IBM Cloud Provider releases. A nightly build job will
trigger a Travis build by publishing a new tag when there are changes for a
release.
