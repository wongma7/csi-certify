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
	"path"
	"regexp"

	"github.com/wongma7/csi-certify/pkg/certify/driver"
	"k8s.io/api/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/podlogs"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"

	. "github.com/onsi/ginkgo"
	_ "github.com/onsi/gomega"
)

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
var _ = Describe("CSI Volumes", func() {
	f := framework.NewDefaultFramework("csi-volumes")

	testfiles.AddFileSource(testfiles.RootFileSource{Root: path.Join(framework.TestContext.RepoRoot, "../../pkg/certify/driver/manifests")})

	var (
		cancel context.CancelFunc
		cs     clientset.Interface
		ns     *v1.Namespace
	)

	BeforeEach(func() {
		ctx, c := context.WithCancel(context.Background())
		cancel = c
		cs = f.ClientSet
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

	driver := driver.Driver()
	config := &testsuites.TestConfig{
		Framework: f,
		Prefix:    "csi",
	}
	// TODO remove
	driver.GetDriverInfo().Config = *config
	Context(testsuites.GetDriverNameWithFeatureTags(driver), func() {
		BeforeEach(func() {
			driver.CreateDriver(config)
		})

		AfterEach(func() {
			driver.CleanupDriver()
		})

		testsuites.SetupTestSuite(driver, config, csiTestSuites, csiTunePattern)
	})
})
