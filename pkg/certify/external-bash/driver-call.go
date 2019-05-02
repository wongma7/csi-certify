package externalBash

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

type BashDriverParameter struct {
}

type testVolume struct {
	volumeAttrib map[string]string
}

var bashDriverParam BashDriverParameter

func init() {
	flag.Var(&bashDriverParam, "bash-testdriver", "name of bashscript that implements all the required testdriver functions")
}

func (b BashDriverParameter) String() string {
	return "<bash testdriver file>"
}

func (b BashDriverParameter) Set(filename string) error {
	RunCustomTestDriver = false
	scriptName = filename

	validTestDriver := checkBashFuncExists("getDriverInfo")
	if validTestDriver == false {
		return errors.Errorf("Invalid TestDriver, must include getDriverInfo() function")
	}

	driver, err := b.getDriverDefinition(scriptName)
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

func (b BashDriverParameter) getDriverDefinition(filename string) (*bashDriver, error) {
	if filename == "" {
		return nil, errors.New("missing file name")
	}

	err, data := execCommand(scriptName, getDriverInfo, "")
	if err != nil {
		return nil, err
	}

	// Some reasonable defaults follow.
	driver := &bashDriver{
		utils.DriverDefinition{
			DriverInfo: testsuites.DriverInfo{
				SupportedFsType: sets.NewString(
					"", // Default fsType
				),
			},
			ClaimSize: "5Gi",
		},
		false,
		false,
	}

	driver.preprovisionedVolumeTestDriver = checkBashFuncExists("createVolume") && checkBashFuncExists("deleteVolume")
	driver.preprovisionedPVTestDriver = driver.preprovisionedVolumeTestDriver

	// TODO: strict checking of the file content once https://github.com/kubernetes/kubernetes/pull/71589
	// or something similar is merged.
	if err := runtime.DecodeInto(legacyscheme.Codecs.UniversalDecoder(), data, driver); err != nil {
		return nil, errors.Wrap(err, filename)
	}

	return driver, nil
}

var _ testsuites.TestDriver = &bashDriver{}

// We have to implement the interface because dynamic PV may or may
// not be supported. driverDefinition.SkipUnsupportedTest checks that
// based on the actual driver definition.
var _ testsuites.DynamicPVTestDriver = &bashDriver{}

// Same for snapshotting.
var _ testsuites.SnapshottableTestDriver = &bashDriver{}

var _ testsuites.PreprovisionedVolumeTestDriver = &bashDriver{}

var _ testsuites.PreprovisionedPVTestDriver = &bashDriver{}

// runtime.DecodeInto needs a runtime.Object but doesn't do any
// deserialization of it and therefore none of the methods below need
// an implementation.
var _ runtime.Object = &bashDriver{}

type bashDriver struct {
	utils.DriverDefinition
	preprovisionedVolumeTestDriver bool
	preprovisionedPVTestDriver     bool
}

func (b *bashDriver) GetDriverInfo() *testsuites.DriverInfo {
	return &b.DriverInfo
}

func (b bashDriver) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
	supported := false
	// TODO (?): add support for more volume types
	switch pattern.VolType {
	case testpatterns.DynamicPV:
		if b.StorageClass.FromName || b.StorageClass.FromFile != "" {
			supported = true
		}
	case testpatterns.PreprovisionedPV:
		if b.preprovisionedPVTestDriver || b.preprovisionedVolumeTestDriver {
			supported = true
		}
	}
	if !supported {
		framework.Skipf("Driver %q does not support volume type %q - skipping", b.DriverInfo.Name, pattern.VolType)
	}

	supported = false
	switch pattern.SnapshotType {
	case "":
		supported = true
	case testpatterns.DynamicCreatedSnapshot:
		if b.SnapshotClass.FromName {
			supported = true
		}
	}
	if !supported {
		framework.Skipf("Driver %q does not support snapshot type %q - skipping", b.DriverInfo.Name, pattern.SnapshotType)
	}
}

func (b bashDriver) CreateVolume(config *testsuites.PerTestConfig, volumeType testpatterns.TestVolType) testsuites.TestVolume {
	f := config.Framework
	ns := f.Namespace

	cvErr, createVolOutput := execCommand(scriptName, createVolume, ns.Name)

	if cvErr != nil {
		fmt.Printf("Unable to create volume\n")
		framework.Failf(cvErr.Error())
	}
	response := make(map[string]string)
	err := json.Unmarshal([]byte(createVolOutput), &response)

	if err != nil {
		panic(err)
	}

	return &testVolume{
		volumeAttrib: response,
	}
}

func (b *bashDriver) GetPersistentVolumeSource(readOnly bool, fsType string, volume testsuites.TestVolume) (*v1.PersistentVolumeSource, *v1.VolumeNodeAffinity) {
	tv, _ := volume.(*testVolume)
	volHandle := b.DriverInfo.Name + "-vol"
	fmt.Printf("\n\nVolume Handle: ------------------- %s\n\n", volHandle)
	return &v1.PersistentVolumeSource{
		CSI: &v1.CSIPersistentVolumeSource{
			Driver:           b.DriverInfo.Name,
			VolumeHandle:     volHandle,
			VolumeAttributes: tv.volumeAttrib,
		},
	}, nil
}

func (b bashDriver) GetDynamicProvisionStorageClass(config *testsuites.PerTestConfig, fsType string) *storagev1.StorageClass {
	f := config.Framework

	if b.StorageClass.FromName {
		provisioner := b.DriverInfo.Name
		parameters := map[string]string{}
		ns := f.Namespace.Name
		suffix := provisioner + "-sc"
		if fsType != "" {
			parameters["csi.storage.k8s.io/fstype"] = fsType
		}

		return testsuites.GetStorageClass(provisioner, parameters, nil, ns, suffix)
	}

	items, err := f.LoadFromManifests(b.StorageClass.FromFile)
	Expect(err).NotTo(HaveOccurred(), "load storage class from %s", b.StorageClass.FromFile)
	Expect(len(items)).To(Equal(1), "exactly one item from %s", b.StorageClass.FromFile)

	err = f.PatchItems(items...)
	Expect(err).NotTo(HaveOccurred(), "patch items")

	sc, ok := items[0].(*storagev1.StorageClass)
	Expect(ok).To(BeTrue(), "storage class from %s", b.StorageClass.FromFile)
	if fsType != "" {
		if sc.Parameters == nil {
			sc.Parameters = map[string]string{}
		}
		sc.Parameters["csi.storage.k8s.io/fstype"] = fsType
	}
	return sc
}

func (b bashDriver) GetSnapshotClass(config *testsuites.PerTestConfig) *unstructured.Unstructured {
	if !b.SnapshotClass.FromName {
		framework.Skipf("Driver %q does not support snapshotting - skipping", b.DriverInfo.Name)
	}

	snapshotter := config.GetUniqueDriverName()
	parameters := map[string]string{}
	ns := config.Framework.Namespace.Name
	suffix := snapshotter + "-vsc"

	return testsuites.GetSnapshotClass(snapshotter, parameters, ns, suffix)
}

func (b *bashDriver) GetClaimSize() string {
	return b.ClaimSize
}

func (b *bashDriver) PrepareTest(f *framework.Framework) (*testsuites.PerTestConfig, func()) {
	config := &testsuites.PerTestConfig{
		Driver:         b,
		Prefix:         "external",
		Framework:      f,
		ClientNodeName: b.ClientNodeName,
	}
	return config, func() {}
}

//Example call: getCommand("nfs", createVolume)
func execCommand(pluginName string, cmdName string, namespaceToUse string) (error, []byte) {
	currentNameSpace, getNameSpaceErr := getCurrentNameSpace()
	if getNameSpaceErr != nil {
		return getNameSpaceErr, nil
	}

	if namespaceToUse == "" {
		namespaceToUse = currentNameSpace
	}

	namespaceErr := setCurrentNameSpace(namespaceToUse)
	if namespaceErr != nil {
		return namespaceErr, nil
	}

	fmt.Printf("Command = %s", ". ../../pkg/certify/external-bash/"+pluginName+" && "+cmdName)

	cmd := exec.Command("bash", "-c", ". ../../pkg/certify/external-bash/"+pluginName+" && "+cmdName)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()

	namespaceErr = setCurrentNameSpace(currentNameSpace)
	if namespaceErr != nil {
		return namespaceErr, nil
	}

	if err != nil {
		fmt.Printf("Unable to execute command: %s \n", cmdName)
		return err, nil
	}

	fmt.Printf("Executing command %s \n %s", cmdName, out.String())
	return nil, out.Bytes()

}

func setCurrentNameSpace(namespace string) error {
	currentContextCmd := exec.Command("bash", "-c", "kubectl config current-context")
	currentContextOutput, err := currentContextCmd.CombinedOutput()

	if err != nil {
		fmt.Printf("Unable to get current context")
		return err
	}

	cmd := exec.Command("bash", "-c", "kubectl config set-context "+strings.TrimSpace(string(currentContextOutput))+" --namespace="+namespace+"")
	output, err := cmd.CombinedOutput()
	fmt.Printf("Kubectl output: %s\n\n", string(output))

	if err != nil {
		fmt.Printf("Unable to execute Kubectl command to set namespace")
		return err
	}

	return nil

}

func getCurrentNameSpace() (string, error) {
	getNameSpaceCmd := exec.Command("bash", "-c", "kubectl config view | grep namespace |  cut -d':' -f 2")
	getNameSpaceOutput, err := getNameSpaceCmd.CombinedOutput()

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(getNameSpaceOutput)), nil
}

func checkBashFuncExists(bashFunc string) bool {

	checkCmd := exec.Command("bash", "-c", ". ../../pkg/certify/external-bash/"+scriptName+" && type "+bashFunc+" &>/dev/null && echo \"found\" || echo \"not found\"")
	checkCmdOutput, err := checkCmd.CombinedOutput()

	if err != nil {
		fmt.Printf("Unable to verify bash function: %s exists", bashFunc)
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
