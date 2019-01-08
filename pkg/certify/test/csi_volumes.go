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
	"regexp"

	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	csiclient "k8s.io/csi-api/pkg/client/clientset/versioned"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/podlogs"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"

	. "github.com/onsi/ginkgo"
	_ "github.com/onsi/gomega"
)

var Driver func(config testsuites.TestConfig) testsuites.TestDriver

// List of testSuites to be executed in below loop
var csiTestSuites = []func() testsuites.TestSuite{
	/*
		testsuites.InitVolumesTestSuite,
		testsuites.InitVolumeIOTestSuite,
		testsuites.InitVolumeModeTestSuite,
		testsuites.InitSubPathTestSuite,
	*/
	testsuites.InitProvisioningTestSuite,
}

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

// This executes testSuites for csi volumes.
var _ = utils.SIGDescribe("CSI Volumes", func() {
	f := framework.NewDefaultFramework("csi-volumes")

	var (
		cancel context.CancelFunc
		cs     clientset.Interface
		csics  csiclient.Interface
		ns     *v1.Namespace
		// Common configuration options for each driver.
		config = testsuites.TestConfig{
			Framework: f,
			Prefix:    "csi",
		}
	)

	BeforeEach(func() {
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		cs = f.ClientSet
		csics = f.CSIClientSet
		ns = f.Namespace

		// Debugging of the following tests heavily depends on the log output
		// of the different containers. Therefore include all of that in log
		// files (when using --report-dir, as in the CI) or the output stream
		// (otherwise).
		to := podlogs.LogOutput{
			StatusWriter: GinkgoWriter,
		}
		if framework.TestContext.ReportDir == "" {
			to.LogWriter = GinkgoWriter
		} else {
			test := CurrentGinkgoTestDescription()
			reg := regexp.MustCompile("[^a-zA-Z0-9_-]+")
			// We end the prefix with a slash to ensure that all logs
			// end up in a directory named after the current test.
			to.LogPathPrefix = framework.TestContext.ReportDir + "/" +
				reg.ReplaceAllString(test.FullTestText, "_") + "/"
		}
		podlogs.CopyAllLogs(ctx, cs, ns.Name, to)

		// pod events are something that the framework already collects itself
		// after a failed test. Logging them live is only useful for interactive
		// debugging, not when we collect reports.
		if framework.TestContext.ReportDir == "" {
			podlogs.WatchPods(ctx, cs, ns.Name, GinkgoWriter)
		}
	})

	AfterEach(func() {
		cancel()
	})

	driver := Driver(config)
	config = driver.GetDriverInfo().Config
	Context(testsuites.GetDriverNameWithFeatureTags(driver), func() {
		BeforeEach(func() {
			// Reset config. The driver might have modified its copy
			// in a previous test.
			driver.GetDriverInfo().Config = config

			driver.CreateDriver(config)
		})

		AfterEach(func() {
			driver.CleanupDriver()
		})

		// why rename to setup if it's still running test suite?
		testsuites.RunTestSuite(f, driver, csiTestSuites, csiTunePattern)
	})
})
