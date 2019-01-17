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
	"path"

	"github.com/wongma7/csi-certify/pkg/certify/driver"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"

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
var _ = utils.SIGDescribe("CSI Volumes", func() {
	testfiles.AddFileSource(testfiles.RootFileSource{Root: path.Join(framework.TestContext.RepoRoot, "../../pkg/certify/driver/manifests")})

	curDriver := driver.Driver()

	Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
		testsuites.DefineTestSuite(curDriver, csiTestSuites, csiTunePattern)
	})
})
