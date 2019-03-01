package certify

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wongma7/csi-certify/pkg/certify/external"
	customTest "github.com/wongma7/csi-certify/pkg/certify/test"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)

	//Only run tests using user's own testdriver implementation(for their CSI driver) only if a driverdefintion YAML was not provided
	if external.RunCustomTestDriver {
		customTest.RunCustomTestDriver()
	}

	RunSpecs(t, "CSI Suite")
}
