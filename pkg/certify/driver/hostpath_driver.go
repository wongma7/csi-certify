package driver

import (
	"fmt"
	"math/rand"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	. "github.com/onsi/ginkgo"
)

var HostPathDriver func() testsuites.TestDriver

func init() {
	HostPathDriver = InitHostPathCSIDriver
}

// hostpathCSI
type hostpathCSIDriver struct {
	driverInfo testsuites.DriverInfo
	manifests  []string
}

func initHostPathCSIDriver(name string, manifests ...string) testsuites.TestDriver {
	return &hostpathCSIDriver{
		driverInfo: testsuites.DriverInfo{
			Name:        name,
			FeatureTag:  "",
			MaxFileSize: testpatterns.FileSizeMedium,
			SupportedFsType: sets.NewString(
				"", // Default fsType
			),
			Capabilities: map[testsuites.Capability]bool{
				testsuites.CapPersistence: true,
			},
		},
		manifests: manifests,
	}
}

var _ testsuites.TestDriver = &hostpathCSIDriver{}
var _ testsuites.DynamicPVTestDriver = &hostpathCSIDriver{}

// InitHostPathCSIDriver returns hostpathCSIDriver that implements TestDriver interface
func InitHostPathCSIDriver() testsuites.TestDriver {
	return initHostPathCSIDriver("csi-hostpath",
		"attacher-rbac.yaml",
		"csi-hostpath-attacher.yaml",
		"csi-hostpathplugin.yaml",
		"csi-hostpath-provisioner.yaml",
		"driver-registrar-rbac.yaml",
		"e2e-test-rbac.yaml",
		"provisioner-rbac.yaml",
	)
}

func (h *hostpathCSIDriver) GetDriverInfo() *testsuites.DriverInfo {
	return &h.driverInfo
}

func (h *hostpathCSIDriver) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
}

func (h *hostpathCSIDriver) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig, fsType string) *storagev1.StorageClass {
	provisioner := config.GetUniqueDriverName()
	parameters := map[string]string{}
	ns := config.Framework.Namespace.Name
	suffix := fmt.Sprintf("%s-sc", provisioner)

	return testsuites.GetStorageClass(provisioner, parameters, nil, ns, suffix)
}

func (h *hostpathCSIDriver) GetClaimSize() string {
	return "5Gi"
}

func (h *hostpathCSIDriver) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	By(fmt.Sprintf("deploying %s driver", h.driverInfo.Name))
	cancel := testsuites.StartPodLogs(f)
	cs := f.ClientSet

	// The hostpath CSI driver only works when everything runs on the same node.
	nodes := framework.GetReadySchedulableNodesOrDie(cs)
	nodeName := nodes.Items[rand.Intn(len(nodes.Items))].Name
	config := &testsuites.PerTestConfig{
		Driver:         h,
		Prefix:         "hostpath",
		Framework:      f,
		ClientNodeName: nodeName,
	}

	// TODO (?): the storage.csi.image.version and storage.csi.image.registry
	// settings are ignored for this test. We could patch the image definitions.
	o := utils.PatchCSIOptions{
		OldDriverName:            h.driverInfo.Name,
		NewDriverName:            config.GetUniqueDriverName(),
		DriverContainerName:      "hostpath",
		DriverContainerArguments: []string{"--drivername=" + config.GetUniqueDriverName()},
		ProvisionerContainerName: "csi-provisioner",
		NodeName:                 nodeName,
	}
	cleanup, err := config.Framework.CreateFromManifests(func(item interface{}) error {
		return utils.PatchCSIDeployment(config.Framework, o, item)
	},
		h.manifests...)
	if err != nil {
		framework.Failf("deploying %s driver: %v", h.driverInfo.Name, err)
	}

	return config, func() {
		By(fmt.Sprintf("uninstalling %s driver", h.driverInfo.Name))
		cleanup()
		cancel()
	}
}
