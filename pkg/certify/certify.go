package certify

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/wongma7/csi-certify/pkg/certify/external"
		customTest "github.com/wongma7/csi-certify/pkg/certify/test"
	"github.com/wongma7/csi-certify/pkg/certify/external-testdriver"
)

func Test(t *testing.T, customTestDriver string) {
	RegisterFailHandler(Fail)

	/*
		Run tests using user's own testDriver implementation if the --testdriver flag is given
		Run tests using user's driverDefinition YAML file if the --driverdef flag is given
		Run tests using an external testDriver (bash script) if the --external-testdriver flag is provided
		If any of the three flags are not given, run all testDriver implementations defined in certify/driver
	*/

	if external.RunCustomTestDriver && external_testdriver.RunCustomTestDriver {
		customTest.RunCustomTestDriver(customTestDriver)
	}

	RunSpecs(t, "CSI Suite")
}
