/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/podlogs"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/testsuites/testdriver"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func csiTunePattern(patterns []testpatterns.TestPattern) []testpatterns.TestPattern {
	tunedPatterns := []testpatterns.TestPattern{}

	for _, pattern := range patterns {
		// Skip inline volume and pre-provsioned PV tests for csi drivers
		if pattern.VolType == testpatterns.InlineVolume || pattern.VolType == testpatterns.PreprovisionedPV {
			continue
		}
		tunedPatterns = append(tunedPatterns, pattern)
	}

	return tunedPatterns
}

var _ = Describe("CSI Volumes", func() {
	f := framework.NewDefaultFramework("csi")

	var (
		cs     clientset.Interface
		ns     *v1.Namespace
		ctx    context.Context
		cancel context.CancelFunc
	)

	BeforeEach(func() {
		cs = f.ClientSet
		ns = f.Namespace
		// These local variables are needed to appease "go vet".
		// It warns about not calling cancel otherwise.
		c, cncl := context.WithCancel(context.Background())
		ctx = c
		cancel = cncl

		// We copy all output from pods directly to the
		// GingkoWriter.
		//
		// When using a more elaborate CI system which uses
		// the --report-dir feature to capture log files, then
		// see k8s.io/kubernetes/test/e2e/storage/csi_volumes.go
		// for an example how the output can also get
		// redirected to log files.
		to := podlogs.LogOutput{
			StatusWriter: GinkgoWriter,
			LogWriter:    GinkgoWriter,
		}
		podlogs.CopyAllLogs(ctx, cs, ns.Name, to)
		podlogs.WatchPods(ctx, cs, ns.Name, GinkgoWriter)
	})

	AfterEach(func() {
		cancel()
	})

	// List of test drivers to be tested against.
	var csiTestDrivers = []func() testdriver.TestDriver{
		// hostpath driver
		func() testdriver.TestDriver {
			return &manifestDriver{
				driverInfo: testdriver.DriverInfo{
					Name:        "csi-hostpath",
					MaxFileSize: testpatterns.FileSizeMedium,
					SupportedFsType: sets.NewString(
						"", // Default fsType
					),
					IsPersistent:       true,
					IsFsGroupSupported: false,
					IsBlockSupported:   false,

					Config: &testdriver.TestConfig{
						Framework: f,
						Prefix:    "csi",
					},
				},
				manifests: []string{
					"test/e2e/storage/manifests/driver-registrar/rbac.yaml",
					"test/e2e/storage/manifests/external-attacher/rbac.yaml",
					"test/e2e/storage/manifests/external-provisioner/rbac.yaml",
					"test/e2e/storage/manifests/hostpath/hostpath/csi-hostpath-attacher.yaml",
					"test/e2e/storage/manifests/hostpath/hostpath/csi-hostpath-provisioner.yaml",
					"test/e2e/storage/manifests/hostpath/hostpath/csi-hostpathplugin.yaml",
					"test/e2e/storage/manifests/hostpath/hostpath/e2e-test-rbac.yaml",
				},
				scManifest: "test/e2e/storage/manifests/hostpath/example/usage/csi-storageclass.yaml",
				// Enable renaming of the driver.
				patchOptions: utils.PatchCSIOptions{
					OldDriverName:            "csi-hostpath",
					NewDriverName:            "csi-hostpath-", // f.UniqueName must be added later
					DriverContainerName:      "hostpath",
					ProvisionerContainerName: "csi-provisioner",
				},
				claimSize: "1Mi",

				// The actual node on which the driver and the test pods run must
				// be set at runtime because it cannot be determined in advance.
				beforeEach: func(m *manifestDriver) {
					nodes := framework.GetReadySchedulableNodesOrDie(cs)
					node := nodes.Items[rand.Intn(len(nodes.Items))]
					m.driverInfo.Config.ClientNodeName = node.Name
					m.patchOptions.NodeName = node.Name

				},
			}
		},
	}

	// List of test suites to be executed for each driver.
	var csiTestSuites = []func() testsuites.TestSuite{
		testsuites.InitVolumesTestSuite,
		testsuites.InitVolumeIOTestSuite,
		testsuites.InitVolumeModeTestSuite,
		testsuites.InitSubPathTestSuite,
		testsuites.InitProvisioningTestSuite,
	}

	for _, initDriver := range csiTestDrivers {
		curDriver := initDriver()
		Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
			driver := curDriver

			BeforeEach(func() {
				// setupDriver
				driver.CreateDriver()
			})

			AfterEach(func() {
				// Cleanup driver
				driver.CleanupDriver()
			})

			testsuites.RunTestSuite(f, driver, csiTestSuites, csiTunePattern)
		})
	}
})

// The manifestDriver implements the test driver interface based on
// a list of yaml files that deploy the driver and a storage class
// for that driver. It supports some additional configuration options
// that control testing (claim size) and driver renaming. With
// driver renaming, tests can run in parallel because each test
// deployes and removes its own driver instance.
type manifestDriver struct {
	driverInfo   testdriver.DriverInfo
	patchOptions utils.PatchCSIOptions
	manifests    []string
	scManifest   string
	claimSize    string
	beforeEach   func(m *manifestDriver)
	cleanup      func()
}

var _ testdriver.TestDriver = &manifestDriver{}
var _ testdriver.DynamicPVTestDriver = &manifestDriver{}

func (m *manifestDriver) GetDriverInfo() *testdriver.DriverInfo {
	return &m.driverInfo
}

func (m *manifestDriver) GetDynamicProvisionStorageClass(fsType string) *storagev1.StorageClass {
	f := m.driverInfo.Config.Framework

	items, err := f.LoadFromManifests(m.scManifest)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(items)).To(Equal(1), "exactly one item from %s", m.scManifest)

	err = f.PatchItems(items...)
	Expect(err).NotTo(HaveOccurred())
	err = utils.PatchCSIDeployment(f, m.finalPatchOptions(), items[0])

	sc, ok := items[0].(*storagev1.StorageClass)
	Expect(ok).To(BeTrue(), "storage class from %s", m.scManifest)
	return sc
}

func (m *manifestDriver) SkipUnsupportedTest(pattern testpatterns.TestPattern) {
}

func (m *manifestDriver) GetClaimSize() string {
	return m.claimSize
}

func (m *manifestDriver) CreateDriver() {
	By(fmt.Sprintf("deploying %s driver", m.driverInfo.Name))
	if m.beforeEach != nil {
		m.beforeEach(m)
	}
	f := m.driverInfo.Config.Framework

	cleanup, err := f.CreateFromManifests(func(item interface{}) error {
		return utils.PatchCSIDeployment(f, m.finalPatchOptions(), item)
	},
		m.manifests...,
	)
	m.cleanup = cleanup
	if err != nil {
		framework.Failf("deploying csi hostpath driver: %v", err)
	}
}

func (m *manifestDriver) CleanupDriver() {
	if m.cleanup != nil {
		By(fmt.Sprintf("uninstalling %s driver", m.driverInfo.Name))
		m.cleanup()
	}
}

func (m *manifestDriver) finalPatchOptions() utils.PatchCSIOptions {
	o := m.patchOptions
	// Unique name not available yet when configuring the driver.
	if strings.HasSuffix(o.NewDriverName, "-") {
		o.NewDriverName += m.driverInfo.Config.Framework.UniqueName
	}
	return o
}
