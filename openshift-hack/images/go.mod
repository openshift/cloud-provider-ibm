module cloud.ibm.com/cloud-provider-vpc-controller

go 1.16

require (
	github.com/IBM/go-sdk-core/v5 v5.6.4
	github.com/IBM/vpc-go-sdk v0.9.0
	github.com/go-openapi/errors v0.20.0 // indirect
	github.com/go-openapi/strfmt v0.20.2
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/leodido/go-urn v1.2.1 // indirect
	github.com/onsi/ginkgo v1.15.0 // indirect
	github.com/onsi/gomega v1.10.5 // indirect
	github.com/stretchr/testify v1.7.0
	google.golang.org/appengine v1.6.7 // indirect
	gopkg.in/gcfg.v1 v1.2.3
	gopkg.in/warnings.v0 v0.1.2 // indirect
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/klog/v2 v2.10.0
)

replace cloud.ibm.com/cloud-provider-vpc-controller => github.com/IBM-Cloud/cloud-provider-vpc-controller v0.0.0-20210902000028-15ad6c21b84c
