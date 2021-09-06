module cloud.ibm.com/cloud-provider-ibm

go 1.16

require (
	cloud.ibm.com/cloud-provider-vpc-controller v0.0.0-00010101000000-000000000000
	github.com/emicklei/go-restful v2.9.6+incompatible // indirect
	github.com/go-openapi/jsonreference v0.19.6 // indirect
	github.com/go-openapi/swag v0.19.15 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/uuid v1.1.5 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	gopkg.in/gcfg.v1 v1.2.3
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/cloud-provider v0.22.0-beta.2
	k8s.io/component-base v0.22.0-beta.2
	k8s.io/klog/v2 v2.10.0
)

replace github.com/coreos/etcd => github.com/coreos/etcd v3.3.25+incompatible

// Use forked version of library with security fix cherry-picked
replace github.com/dgrijalva/jwt-go v3.2.0+incompatible => github.com/form3tech-oss/jwt-go v3.2.1+incompatible

replace cloud.ibm.com/cloud-provider-vpc-controller => github.com/IBM-Cloud/cloud-provider-vpc-controller v0.0.0-20210902000028-15ad6c21b84c
