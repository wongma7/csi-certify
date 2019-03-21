package external_testdriver

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/wongma7/csi-certify/pkg/certify/utils"
	"k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"os/exec"
	"strings"
)

const (
	getDriverInfo = "getDriverInfo"
	createVolume  = "createVolume"
)

var RunCustomTestDriver = true
var scriptName = ""
var currentNameSpace = "default"
var currentDriver *driverDefinition

var validTestDriver = false
var preprovisionedVolumeTestDriver = false
var preprovisionedPVTestDriver = false

type DriverCall struct {
}

type testVolume struct {
	volumeAttrib map[string]string
}

var driverCall DriverCall

func init() {
	flag.Var(&driverCall, "external-testdriver", "name of bashscript that implements all the required testdriver functions")
}

func (d DriverCall) String() string {
	return "<bash testdriver file>"
}

func (d DriverCall) Set(filename string) error {
	RunCustomTestDriver = false
	scriptName = filename
	driver, err := d.getDriverDefinition(scriptName)
	getTestDriverType()

	if err != nil {
		return err
	}

	if validTestDriver == false {
		framework.Failf("Invalid TestDriver, must include getDriverInfo() function")
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

func (d DriverCall) getDriverDefinition(filename string) (*driverDefinition, error) {
	if filename == "" {
		return nil, errors.New("missing file name")
	}

	data := execCommand(scriptName, getDriverInfo)

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
	if err := runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), data, driver); err != nil {
		return nil, errors.Wrap(err, filename)
	}

	currentDriver = driver
	return driver, nil
}

var _ testsuites.TestDriver = &driverDefinition{}

// We have to implement the interface because dynamic PV may or may
// not be supported. driverDefinition.SkipUnsupportedTest checks that
// based on the actual driver definition.
var _ testsuites.DynamicPVTestDriver = &driverDefinition{}

// Same for snapshotting.
var _ testsuites.SnapshottableTestDriver = &driverDefinition{}

var _ testsuites.PreprovisionedVolumeTestDriver = &driverDefinition{}

var _ testsuites.PreprovisionedPVTestDriver = &driverDefinition{}

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
	case testpatterns.PreprovisionedPV:
		if preprovisionedPVTestDriver || preprovisionedVolumeTestDriver {
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

func (d driverDefinition) CreateVolume(config *testsuites.PerTestConfig, volumeType testpatterns.TestVolType) testsuites.TestVolume {
	f := config.Framework
	ns := f.Namespace

	//Set the current namespace
	currentNameSpace = ns.Name

	createVolOutput := execCommand(scriptName, createVolume)
	response := make(map[string]string)
	err := json.Unmarshal([]byte(createVolOutput), &response)

	if err != nil {
		panic(err)
	}

	fmt.Printf("default response --------, %+v", response)

	//Need someway to wait for objects user creates to be created, before proceeding
	//Should probably use a timeout
	toreturn := &testVolume{
		volumeAttrib: response,
	}

	fmt.Printf("Returning response: %v", toreturn)
	return toreturn
}

func (d *driverDefinition) GetPersistentVolumeSource(readOnly bool, fsType string, volume testsuites.TestVolume) (*v1.PersistentVolumeSource, *v1.VolumeNodeAffinity) {
	tv, _ := volume.(*testVolume)
	volHandle := currentDriver.DriverInfo.Name + "-vol"
	fmt.Printf("\n\nCurrent NameSpace: ------------------- %s\n\n", currentNameSpace)
	fmt.Printf("\n\nVolume Handle: ------------------- %s\n\n", volHandle)
	return &v1.PersistentVolumeSource{
		CSI: &v1.CSIPersistentVolumeSource{
			Driver:           d.DriverInfo.Name,
			VolumeHandle:     volHandle,
			VolumeAttributes: tv.volumeAttrib,
		},
	}, nil
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

//Example call: getCommand("nfs", createVolume)
func execCommand(pluginName string, cmdName string) []byte {
	setCurrentNameSpace(currentNameSpace)

	fmt.Printf("Command = %s", ". ../../pkg/certify/external-testdriver/"+pluginName+" && "+cmdName)

	cmd := exec.Command("bash", "-c", ". ../../pkg/certify/external-testdriver/"+pluginName+" && "+cmdName)
	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Unable to execute command: %s \n", cmdName)
		fmt.Printf(err.Error())
	} else {
		fmt.Printf("Executing command %s \n %s", cmdName, out.String())
		return out.Bytes()
	}

	setCurrentNameSpace("default")

	return nil
}

func setCurrentNameSpace(namespace string) error {
	currentContextCmd := exec.Command("bash", "-c", "kubectl config current-context")
	currentContextOutput, err := currentContextCmd.CombinedOutput()

	if err != nil {
		fmt.Printf("Unable to get current context")
		fmt.Printf(err.Error())
		return err
	}

	cmd := exec.Command("bash", "-c", "kubectl config set-context "+strings.TrimSpace(string(currentContextOutput))+" --namespace="+namespace+"")
	output, err := cmd.CombinedOutput()
	fmt.Printf("Kubectl output: %s\n\n", string(output))

	if err != nil {
		fmt.Printf("Unable to execute Kubectl command to set namespace")
		fmt.Printf(err.Error())
		return err
	}

	return nil

}

func getTestDriverType() {
	validTestDriver = checkBashFuncExists("getDriverInfo")
	preprovisionedVolumeTestDriver = checkBashFuncExists("createVolume") && checkBashFuncExists("deleteVolume")
	preprovisionedPVTestDriver = preprovisionedVolumeTestDriver
}

func checkBashFuncExists(bashFunc string) bool {

	checkCmd := exec.Command("bash", "-c", ". ../../pkg/certify/external-testdriver/"+scriptName+" && type "+bashFunc+" &>/dev/null && echo \"found\" || echo \"not found\"")
	checkCmdOutput, err := checkCmd.CombinedOutput()

	if err != nil {
		fmt.Printf("Unable to check bash func")
		fmt.Printf(err.Error())
	}

	if strings.TrimSpace(string(checkCmdOutput)) == "found" {
		return true
	}

	return false
}

func (v testVolume) DeleteVolume() {
	//Delete the volume
}
