package certify

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wongma7/csi-certify/pkg/certify/external"
	customTest "github.com/wongma7/csi-certify/pkg/certify/test"
	"github.com/wongma7/csi-certify/pkg/certify/utils"
	"k8s.io/kubernetes/test/e2e/framework"
)

func Test(t *testing.T, customTestDriver string) {
	RegisterFailHandler(Fail)
	//Only run tests using user's own testdriver implementation(for their CSI driver) only if a driverdefintion YAML was not provided
	if external.RunCustomTestDriver && utils.CSITestDrivers[customTestDriver] != nil {
		customTest.RunCustomTestDriver(customTestDriver)
	} else {
		framework.Failf("Insufficient arguments to run e2e tests")
	}

	RunSpecs(t, "CSI Suite")
}
