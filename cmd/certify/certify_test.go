package main

import (
	"flag"
	"testing"

	"github.com/wongma7/csi-certify/pkg/certify"
	"k8s.io/kubernetes/test/e2e/framework"
)

func init() {
	framework.HandleFlags()
	framework.AfterReadingAllFlags(&framework.TestContext)
}

func Test(t *testing.T) {
	flag.Parse()
	certify.Test(t)
}
