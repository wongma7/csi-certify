package external

import (
	"flag"
	"io/ioutil"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

// List of testSuites to be executed for each external driver.
var csiTestSuites = []func() testsuites.TestSuite{
	//testsuites.InitVolumesTestSuite,
	//testsuites.InitVolumeIOTestSuite,
	//testsuites.InitVolumeModeTestSuite,
	//testsuites.InitSubPathTestSuite,
	testsuites.InitProvisioningTestSuite,
}

type TestDriverParameter struct {
}

var testDriverParam TestDriverParameter

func init() {
	flag.Var(&testDriverParam, "storage.testdriver", "name of a .yaml or .json file that defines a driver for storage testing, can be used more than once")
}

func (t TestDriverParameter) String() string {
	return "<.yaml or .json file>"
}

func (t TestDriverParameter) Set(filename string) error {
	driver, err := t.loadDriverDefinition(filename)
	if err != nil {
		return err
	}

	Describe("External Storage "+testsuites.GetDriverNameWithFeatureTags(driver), func() {
		testsuites.DefineTestSuite(driver, csiTestSuites)
	})

	return nil
}

func (t TestDriverParameter) loadDriverDefinition(filename string) (*DriverDefinition, error) {
	if filename == "" {
		return nil, errors.New("missing file name")
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	// Some reasonable defaults follow.
	driver := &DriverDefinition{
		DriverInfo: testsuites.DriverInfo{
			SupportedFsType: sets.NewString(
				"", // Default fsType
			),
		},
		ClaimSize:    "5Gi",
		StorageClass: "default",
	}
	// TODO: strict checking of the file content once https://github.com/kubernetes/kubernetes/pull/71589
	// or something similar is merged.
	if err := runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), data, driver); err != nil {
		return nil, errors.Wrap(err, filename)
	}

	return driver, nil
}

var _ testsuites.TestDriver = &DriverDefinition{}

// We have to implement the interface because dynamic PV may or may
// not be supported. driverDefinition.SkipUnsupportedTest checks that
// based on the actual driver definition.
var _ testsuites.DynamicPVTestDriver = &DriverDefinition{}

// Needed only for deserialization which isn't done, therefore the
// functions below can be empty stubs.
var _ runtime.Object = &DriverDefinition{}

// DriverDefinition needs to be filled in via a .yaml or .json
// file. It's methods then implement the TestDriver interface, using
// nothing but the information in this struct.
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

func (d *DriverDefinition) DeepCopyObject() runtime.Object {
	return nil
}

func (d *DriverDefinition) GetObjectKind() schema.ObjectKind {
	return nil
}

func (d *DriverDefinition) GetDriverInfo() *testsuites.DriverInfo {
	return &d.DriverInfo
}

func (d *DriverDefinition) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
	supported := false
	// TODO (?): add support for more volume types
	switch pattern.VolType {
	case testpatterns.DynamicPV:
		if d.StorageClass != "" {
			supported = true
		}
	}
	if !supported {
		framework.Skipf("Driver %q does not support volume type %q - skipping", d.DriverInfo.Name, pattern.VolType)
	}
}

func (d *DriverDefinition) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig, fsType string) *storagev1.StorageClass {

	if d.StorageClass == "default" {
		provisioner := d.DriverInfo.Name
		parameters := map[string]string{}
		ns := config.Framework.Namespace.Name
		suffix := provisioner + "-sc"

		return testsuites.GetStorageClass(provisioner, parameters, nil, ns, suffix)
	}

	f := config.Framework

	items, err := f.LoadFromManifests(d.StorageClass)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(items)).To(Equal(1), "exactly one item from %s", d.StorageClass)

	err = f.PatchItems(items...)
	Expect(err).NotTo(HaveOccurred())

	sc, ok := items[0].(*storagev1.StorageClass)
	Expect(ok).To(BeTrue(), "storage class from %s", d.StorageClass)
	return sc
}

func (d *DriverDefinition) GetClaimSize() string {
	return d.ClaimSize
}

func (d *DriverDefinition) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	config := &testsuites.PerTestConfig{
		Driver:         d,
		Prefix:         d.ShortName,
		Framework:      f,
		ClientNodeName: d.ClientNodeName,
	}

	return config, func() {}

}

func (d *DriverDefinition) CleanupDriver() {
}
