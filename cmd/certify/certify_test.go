package main

import (
	"flag"
	"testing"

	"github.com/wongma7/csi-certify/pkg/certify"
)

func Test(t *testing.T) {
	flag.Parse()
	certify.Test(t)
}
