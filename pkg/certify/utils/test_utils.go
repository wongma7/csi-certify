package utils

import (
	"github.com/wongma7/csi-certify/pkg/certify/driver"
	"k8s.io/kubernetes/test/e2e/storage/testsuites"
)

// List of testSuites to be executed
var CSITestSuites = []func() testsuites.TestSuite{
	testsuites.InitVolumesTestSuite,
	testsuites.InitVolumeIOTestSuite,
	testsuites.InitVolumeModeTestSuite,
	testsuites.InitSubPathTestSuite,
	testsuites.InitProvisioningTestSuite,
}

var CSITestDrivers = map[string]func() testsuites.TestDriver{
	"hostpath": driver.InitHostPathCSIDriver,
	"nfs":      driver.InitNFSDriver,
}
