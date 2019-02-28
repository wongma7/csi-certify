# Certifying CSI Plugins

## Running E2E Tests on your CSI Driver

To be able to run csi-certify, a YAML file that defines a DriverDefinition object is required. This YAML file would provide required information such as Driver Name, Supported Fs Type, Supported Mount Option, etc. The Driver Name is then used by csi-certify to identify which driver in the cluster the e2e tests will be ran against, while the other parameters are used to determine which test cases are valid for the given driver. 

The DriverDefiniton object is defined as: 
```
type DriverDefinition struct {
	// DriverInfo is the static information that the storage testsuite
	// expects from a test driver. See test/e2e/storage/testsuites/testdriver.go
	// for details. The only field with a non-zero default is the list of
	// supported file systems (SupportedFsType): it is set so that tests using
	// the default file system are enabled.
	DriverInfo testsuites.DriverInfo

	// ShortName is used to create unique names for test cases and test resources.
	ShortName string

	// StorageClass is required for enabling dynamic provisioning tests.
	// When set to "default", a StorageClass with DriverInfo.Name as provisioner is used.
	// Everything else is treated like the filename of a .yaml or .json file
	// that defines a StorageClass.
	StorageClass string

	// ClaimSize defines the desired size of dynamically
	// provisioned volumes. Default is "5GiB".
	ClaimSize string

	// ClientNodeName selects a specific node for scheduling test pods.
	// Can be left empty.
	ClientNodeName string
}
```

and the DriverInfo object is defined as:

```
// DriverInfo represents static information about a TestDriver.
type DriverInfo struct {
	Name       string // Name of the driver
	FeatureTag string // FeatureTag for the driver

	MaxFileSize          int64               // Max file size to be tested for this driver
	SupportedFsType      sets.String         // Map of string for supported fs type
	SupportedMountOption sets.String         // Map of string for supported mount option
	RequiredMountOption  sets.String         // Map of string for required mount option (Optional)
	Capabilities         map[Capability]bool // Map that represents plugin capabilities
}
```

Example: csi-certify can be ran on the HostPath Driver using [this DriverDefinition YAML file](https://github.com/wongma7/csi-certify/blob/mathusan-out-of-tree-POC/pkg/certify/external/testfiles/driver-def.yaml)


In some cases just providing a DriverDefinition YAML is not sufficient. 
 1) The default storage class created may not be enough. The default storage class is simply a StorageClass with provisioner field set to the driver’s name. You can use your own custom StorageClass by making a StorageClass yaml file and passing the name of that file in the StorageClass field of the DriverDefinition YAML file. csi-certify will then use this YAML to create the StorageClass that is required to test dynamic provisioning on your driver.
 
 2) If simply deploying your driver through YAML files is not enough, users would need to write their own [TestDriver](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/testsuites/testdriver.go#L31), similar to the [HostPath Test Driver](https://github.com/wongma7/csi-certify/blob/refactor/pkg/certify/driver/hostpath_driver.go). This test driver can be placed in the [pkg/certify/driver](https://github.com/wongma7/csi-certify/tree/master/pkg/certify/driver) directory. (WIP, need to add a template/skeleton file so this is easier to do).
 
Notes:
 - DriverDefinition is WIP: See [PR] (https://github.com/kubernetes/kubernetes/pull/72836/files). 
 
### How to run the e2e tests

#### Prerequisites

 * A Kubernetes v1.12+ Cluster
 * [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl) 
 
 This part is up to you since it all depends on what kind of backend your driver requires. Here is an example of installing the HostPath driver with a local cluster created using local-up-cluster.sh 
 
 Clone Kubernetes: `git clone https://github.com/kubernetes/kubernetes.git` 
 
 Stand up cluster by running: `ALLOW_PRIVILEGED=1 hack/local-up-cluster.sh` 
 
 In a seperate shell (After cd to csi-certify repo): 
 ```
    export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
    kubectl create -f pkg/driver/manifests
 ```
 This would install the HostPath Driver on a local kubernetes cluster, which you can check by running `kubectl get pods` 
 
To run e2e tests using a DriverDefintion YAML: 
```
go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig --driverdef=<Path To Driver Info YAML>
``` 

To run e2e tests using the TestDriver that you wrote: 
```
go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig
``` 

Since we have both the DriverDefinition YAML and a TestDriver written for the HostPath driver, we can run it using either way. The command to run e2e tests on the HostPath CSI driver by passing a DriverDefinition YAML file would be: 

```
go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig --driverdef=../../pkg/certify/external/testfiles/driver-def.yaml
```

 
