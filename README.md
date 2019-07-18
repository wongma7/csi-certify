
# Certifying CSI Plugins -> MOVED TO [yard-turkey/csi-certify](https://github.com/yard-turkey/csi-certify) 

## Running E2E Tests on your CSI Plugin

#### Prerequisites

 * A Kubernetes v1.12+ Cluster
 * [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-kubectl) 
 
 This part is up to you since it all depends on what kind of backend your CSI plugin requires. This is how you would create a local cluster using local-up-cluster.sh:
 
 Clone Kubernetes: `git clone https://github.com/kubernetes/kubernetes.git` 
 
 For Fedora/RHEL local clusters, you probably need to do the following: 
 ```
 sudo chown -R $USER:$USER /var/run/kubernetes/
 sudo chown -R $USER:$USER /var/lib/kubelet 
 sudo chcon -R -t svirt_sandbox_file_t /var/lib/kubelet
 ```
 Stand up cluster by running: `ALLOW_PRIVILEGED=1 hack/local-up-cluster.sh` 
 
 In a seperate shell (After cd to csi-certify repo): 
 ```
    export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
 ``` 

To be able to run csi-certify, a YAML file that defines a DriverDefinition object is required. This YAML file would provide required information such as Plugin Name, Supported Fs Type, Supported Mount Option, etc. The Plugin Name is then used by csi-certify to identify which driver in the cluster the e2e tests will be ran against, while the other parameters are used to determine which test cases are valid for the given plugin. 

The DriverDefiniton object is defined as: 
```
// DriverDefinition needs to be filled in via a .yaml or .json
// file. It's methods then implement the TestDriver interface, using
// nothing but the information in this struct.
type driverDefinition struct {
	// DriverInfo is the static information that the storage testsuite
	// expects from a test driver. See test/e2e/storage/testsuites/testdriver.go
	// for details. The only field with a non-zero default is the list of
	// supported file systems (SupportedFsType): it is set so that tests using
	// the default file system are enabled.
	DriverInfo testsuites.DriverInfo

	// ShortName is used to create unique names for test cases and test resources.
	ShortName string

	// StorageClass must be set to enable dynamic provisioning tests.
	// The default is to not run those tests.
	StorageClass struct {
		// FromName set to true enables the usage of a storage
		// class with DriverInfo.Name as provisioner and no
		// parameters.
		FromName bool

		// FromFile is used only when FromName is false.  It
		// loads a storage class from the given .yaml or .json
		// file. File names are resolved by the
		// framework.testfiles package, which typically means
		// that they can be absolute or relative to the test
		// suite's --repo-root parameter.
		//
		// This can be used when the storage class is meant to have
		// additional parameters.
		FromFile string
	}

	// SnapshotClass must be set to enable snapshotting tests.
	// The default is to not run those tests.
	SnapshotClass struct {
		// FromName set to true enables the usage of a
		// snapshotter class with DriverInfo.Name as provisioner.
		FromName bool

		// TODO (?): load from file
	}

	// ClaimSize defines the desired size of dynamically
	// provisioned volumes. Default is "5GiB".
	ClaimSize string

	// ClientNodeName selects a specific node for scheduling test pods.
	// Can be left empty. Most drivers should not need this and instead
	// use topology to ensure that pods land on the right node(s).
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

Example: csi-certify can be ran on the HostPath CSI Plugin using [this DriverDefinition YAML file](https://github.com/wongma7/csi-certify/blob/master/pkg/certify/external/driver-def.yaml)


In some cases just providing a DriverDefinition YAML is not sufficient. 
 1) The default storage class created may not be enough. The default storage class is simply a StorageClass with provisioner field set to the plugin’s name. You can use your own custom StorageClass by making a StorageClass yaml file and passing the name of that file in the StorageClass field of the DriverDefinition YAML file. csi-certify will then use this YAML to create the StorageClass that is required to test dynamic provisioning on your plugin.
 
 2) If simply deploying your plugin through YAML files is not enough, users would need to write their own [TestDriver](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/testsuites/testdriver.go#L31), similar to the [HostPath TestDriver](https://github.com/wongma7/csi-certify/blob/master/pkg/certify/driver/hostpath_driver.go). This testdriver can be placed in the [pkg/certify/driver](https://github.com/wongma7/csi-certify/tree/master/pkg/certify/driver) directory. (WIP, need to add a template/skeleton file so this is easier to do).
 
### How to run the e2e tests
 
To run e2e tests using a DriverDefintion YAML: 
```
./certify --driverdef=<Path To Driver Info YAML> --kubeconfig=<Path to KubeConfig file> --skip=<Regex to Skip Tests (not required)>
``` 

To run e2e tests using the TestDriver that you wrote: 
```
go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig --testdriver=<Name of TestDriver that you want to run> -timeout=0
``` 

Since we have both the DriverDefinition YAML and a TestDriver written for the HostPath plugin, we can run it using either way. The command to run e2e tests on the HostPath CSI plugin by passing a DriverDefinition YAML file would be: 

```
kubectl create -f pkg/certify/driver/manifests/hostpath #To first Install the hostpath driver onto your cluster

./certify --driverdef=pkg/certify/external/driver-def.yaml --kubeconfig=/var/run/kubernetes/admin.kubeconfig
```

To run e2e tests on the HostPath CSI plugin using the implemented TestDriver: 
```
go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig --testdriver=hostpath -timeout=0
```

## NFS TestDriver Example

An [NFS TestDriver](https://github.com/wongma7/csi-certify/blob/master/pkg/certify/driver/nfs_driver.go) was implemented to run e2e tests on the [NFS CSI Plugin](https://github.com/kubernetes-csi/csi-driver-nfs)

### Notes

For any testdriver the required methods that need to be implemented are (Since it implements the [TestDriver Interface](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/testsuites/testdriver.go#L31)):
 - `GetDriverInfo() *testsuites.DriverInfo`
 - `SkipUnsupportedTest(pattern testpatterns.TestPattern)`
 - `PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func())` 
 
Depending on your plugin's specs, it should  also implement other interaces defined [here](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/testsuites/testdriver.go). For example the NFS TestDriver implements the [PreprovisionedVolumeTestDriver](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/testsuites/testdriver.go#L61), and [PreprovisionedPVTestDriver](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/testsuites/testdriver.go#L78). So you would need to implement additional methods such as `CreateVolume`, `GetDynamicProvisionStorageClass`, etc. The HostPath TestDriver implements [DynamicPVTestDriver](https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/testsuites/testdriver.go#L87) because it supports dynamic provioning, and dynamic provisioning tests will be ran (instead of skipped).
 
 To be able to test the NFS CSI plugin, you would need to setup an NFS server that could be used by the tests. This is done in the `CreateVolume` [method](https://github.com/wongma7/csi-certify/blob/master/pkg/certify/driver/nfs_driver.go#L103) which is called once for each test case. CreateVolume creates a server pod with an NFS server image. The pod and the server IP are then returned so that it is usable by the tests.

### Running e2e tests on the NFS CSI Plugin

Have your kubernetes cluster setup, as mentioned in the Prerequisites

Since an NFS CSI docker image is yet to be pushed into K8's official Quay repo, the [YAML files](https://github.com/wongma7/csi-certify/tree/master/pkg/certify/driver/manifests/nfs) use this image: `quay.io/mathu97/nfsplugin:v1.0.0`

If you want, you could build your own image of the NFS plugin and edit the manifests files to use it.

Run e2e tests by specifying the testdriver as nfs, by passing the --testdriver parameter
```
go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig --testdriver=nfs -timeout=0
```

### Using csi-cert in a CI for your CSI Plugin 

Here is an [example CI](https://travis-ci.org/mathu97/csi-driver-nfs/builds/510069940), which is setup such that the e2e tests are run on a [fork of the NFS CSI plugin](https://github.com/mathu97/csi-driver-nfs).

 - Travis clones kubernetes and sets up a local kubernetes cluster using local-up-cluster (exactly how we do it for hostpath/nfs plugins above)
 - [Travis CI config](https://travis-ci.org/mathu97/csi-driver-nfs/jobs/510069941/config)
 - Note: Disruptive tests are disabled by providing the flag `-ginkgo.skip="Disruptive"` as those are the only failing tests, and they are expected to fail.

