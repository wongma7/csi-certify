package certify

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	_ "github.com/wongma7/csi-certify/pkg/certify/test"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "CSI Suite")
}
