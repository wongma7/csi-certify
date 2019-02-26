package utils

import (
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

// List of testSuites to be executed
var CSITestSuites = []func() testsuites.TestSuite{
	//testsuites.InitVolumesTestSuite,
	//testsuites.InitVolumeIOTestSuite,
	//testsuites.InitVolumeModeTestSuite,
	//testsuites.InitSubPathTestSuite,
	testsuites.InitProvisioningTestSuite,
}
