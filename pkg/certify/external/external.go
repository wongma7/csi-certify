package external

import (
	"flag"
	"io/ioutil"

	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wongma7/csi-certify/pkg/certify/utils"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

type DriverDefParameter struct {
}

var RunCustomTestDriver = true
var driverDefParam DriverDefParameter

func init() {
	flag.Var(&driverDefParam, "driverdef", "name of a .yaml or .json file that defines a driver for storage testing, can be used more than once")
}

func (d DriverDefParameter) String() string {
	return "<.yaml or .json file>"
}

func (d DriverDefParameter) Set(filename string) error {
	RunCustomTestDriver = false
	driver, err := d.loadDriverDefinition(filename)
	if err != nil {
		return err
	}

	Describe("External Storage "+testsuites.GetDriverNameWithFeatureTags(driver), func() {
		testsuites.DefineTestSuite(driver, utils.CSITestSuites)
	})

	return nil
}

func (d DriverDefParameter) loadDriverDefinition(filename string) (*DriverDefinition, error) {
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
	f := config.Framework

	if d.StorageClass == "default" {
		provisioner := d.DriverInfo.Name
		parameters := map[string]string{}
		ns := f.Namespace.Name
		suffix := provisioner + "-sc"
		if fsType != "" {
			parameters["csi.storage.k8s.io/fstype"] = fsType
		}

		return testsuites.GetStorageClass(provisioner, parameters, nil, ns, suffix)
	}

	items, err := f.LoadFromManifests(d.StorageClass)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(items)).To(Equal(1), "exactly one item from %s", d.StorageClass)

	err = f.PatchItems(items...)
	Expect(err).NotTo(HaveOccurred())

	sc, ok := items[0].(*storagev1.StorageClass)
	Expect(ok).To(BeTrue(), "storage class from %s", d.StorageClass)
	if fsType != "" {
		if sc.Parameters == nil {
			sc.Parameters = map[string]string{}
		}
		sc.Parameters["csi.storage.k8s.io/fstype"] = fsType
	}
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
