package certify

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wongma7/csi-certify/pkg/certify/external"
	customTest "github.com/wongma7/csi-certify/pkg/certify/test"
)

func Test(t *testing.T, customTestDriver string) {
	RegisterFailHandler(Fail)

	/*
		Run tests using user's own testDriver implementation if the --testdriver flag is given
		Run tests using user's driverDefinition YAML file if the --driverdef flag is given
		If either of the two flags are not given, run all testDriver implementations defined
	*/

	if external.RunCustomTestDriver {
		customTest.RunCustomTestDriver(customTestDriver)
	}

	RunSpecs(t, "CSI Suite")
}
