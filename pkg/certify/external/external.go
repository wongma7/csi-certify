package external

import (
	"flag"
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wongma7/csi-certify/pkg/certify/utils"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"io/ioutil"
	"fmt"
)

type DriverDefParameter struct {
}

var RunCustomTestDriver = true
var DriverDefParam DriverDefParameter

func init() {
	flag.Var(&DriverDefParam, "driverdef", "name of a .yaml or .json file that defines a driver for storage testing, can be used more than once")
}

func (d DriverDefParameter) String() string {
	return "<.yaml or .json file>"
}

func (d DriverDefParameter) Set(filename string) error {
	fmt.Printf("Enters EXTERNAL --------- HERE\n\n")
	RunCustomTestDriver = false
	driver, err := d.loadDriverDefinition(filename)
	if err != nil {
		return err
	}
	if driver.DriverInfo.Name == "" {
		return errors.Errorf("%q: DriverInfo.Name not set", filename)
	}

	description := "External Storage " + testsuites.GetDriverNameWithFeatureTags(driver)
	Describe(description, func() {
		testsuites.DefineTestSuite(driver, utils.CSITestSuites)
	})

	return nil

}

func (d DriverDefParameter) loadDriverDefinition(filename string) (*driverDefinition, error) {
	if filename == "" {
		return nil, errors.New("missing file name")
	}
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	// Some reasonable defaults follow.
	driver := &driverDefinition{
		utils.DriverDefinition{
			DriverInfo: testsuites.DriverInfo{
				SupportedFsType: sets.NewString(
					"", // Default fsType
				),
			},
			ClaimSize: "5Gi",
		},
	}
	// TODO: strict checking of the file content once https://github.com/kubernetes/kubernetes/pull/71589
	// or something similar is merged.
	if err := runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), data, &driver.DriverDefinition); err != nil {
		return nil, errors.Wrap(err, filename)
	}
	return driver, nil
}

var _ testsuites.TestDriver = &driverDefinition{}

// We have to implement the interface because dynamic PV may or may
// not be supported. driverDefinition.SkipUnsupportedTest checks that
// based on the actual driver definition.
var _ testsuites.DynamicPVTestDriver = &driverDefinition{}

// Same for snapshotting.
var _ testsuites.SnapshottableTestDriver = &driverDefinition{}

// runtime.DecodeInto needs a runtime.Object but doesn't do any
// deserialization of it and therefore none of the methods below need
// an implementation.
var _ runtime.Object = &driverDefinition{}

type driverDefinition struct {
	utils.DriverDefinition
}

func (d *driverDefinition) GetDriverInfo() *testsuites.DriverInfo {
	return &d.DriverInfo
}

func (d driverDefinition) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
	supported := false
	// TODO (?): add support for more volume types
	switch pattern.VolType {
	case testpatterns.DynamicPV:
		if d.StorageClass.FromName || d.StorageClass.FromFile != "" {
			supported = true
		}
	}
	if !supported {
		framework.Skipf("Driver %q does not support volume type %q - skipping", d.DriverInfo.Name, pattern.VolType)
	}

	supported = false
	switch pattern.SnapshotType {
	case "":
		supported = true
	case testpatterns.DynamicCreatedSnapshot:
		if d.SnapshotClass.FromName {
			supported = true
		}
	}
	if !supported {
		framework.Skipf("Driver %q does not support snapshot type %q - skipping", d.DriverInfo.Name, pattern.SnapshotType)
	}
}

func (d driverDefinition) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig, fsType string) *storagev1.StorageClass {
	f := config.Framework

	if d.StorageClass.FromName {
		provisioner := d.DriverInfo.Name
		parameters := map[string]string{}
		ns := f.Namespace.Name
		suffix := provisioner + "-sc"
		if fsType != "" {
			parameters["csi.storage.k8s.io/fstype"] = fsType
		}

		return testsuites.GetStorageClass(provisioner, parameters, nil, ns, suffix)
	}

	items, err := f.LoadFromManifests(d.StorageClass.FromFile)
	Expect(err).NotTo(HaveOccurred(), "load storage class from %s", d.StorageClass.FromFile)
	Expect(len(items)).To(Equal(1), "exactly one item from %s", d.StorageClass.FromFile)

	err = f.PatchItems(items...)
	Expect(err).NotTo(HaveOccurred(), "patch items")

	sc, ok := items[0].(*storagev1.StorageClass)
	Expect(ok).To(BeTrue(), "storage class from %s", d.StorageClass.FromFile)
	if fsType != "" {
		if sc.Parameters == nil {
			sc.Parameters = map[string]string{}
		}
		sc.Parameters["csi.storage.k8s.io/fstype"] = fsType
	}
	return sc
}

func (d driverDefinition) GetSnapshotClass(config *testsuites.PerTestConfig) *unstructured.Unstructured {
	if !d.SnapshotClass.FromName {
		framework.Skipf("Driver %q does not support snapshotting - skipping", d.DriverInfo.Name)
	}

	snapshotter := config.GetUniqueDriverName()
	parameters := map[string]string{}
	ns := config.Framework.Namespace.Name
	suffix := snapshotter + "-vsc"

	return testsuites.GetSnapshotClass(snapshotter, parameters, ns, suffix)
}

func (d *driverDefinition) GetClaimSize() string {
	return d.ClaimSize
}

func (d *driverDefinition) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	config := &testsuites.PerTestConfig{
		Driver:         d,
		Prefix:         "external",
		Framework:      f,
		ClientNodeName: d.ClientNodeName,
	}
	return config, func() {}
}
