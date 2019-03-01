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
	. "github.com/onsi/ginkgo"
	_ "github.com/onsi/gomega"
	"github.com/wongma7/csi-certify/pkg/certify/driver"
	testUtils "github.com/wongma7/csi-certify/pkg/certify/utils"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
	"k8s.io/kubernetes/test/e2e/storage/utils"
	"path"
)

// This executes testSuites for csi volumes.
func RunCustomTestDriver() {
	var _ = utils.SIGDescribe("CSI Volumes", func() {
		testfiles.AddFileSource(testfiles.RootFileSource{Root: path.Join(framework.TestContext.RepoRoot, "./pkg/certify/driver/manifests")})

		curDriver := driver.Driver()

		Context(testsuites.GetDriverNameWithFeatureTags(curDriver), func() {
			testsuites.DefineTestSuite(curDriver, testUtils.CSITestSuites)
		})
	})

}
