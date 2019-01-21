WIP/POC trying out the APIs being added by https://github.com/kubernetes/kubernetes/pull/72434. Basically, the same as https://github.com/pohly/csi-e2e/ but from scratch to mimic the csi-sanity layout a bit closer and see how easy setting up and running the existing test suites out-of-tree against some arbitrary driver (TestDriver) is.

https://github.com/wongma7/csi-certify/blob/master/pkg/certify/test/csi_volumes.go is where the test specs are defined, same as https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/csi_volumes.go. All that's needed is to pass a TestDriver to DefineTestSuite. The driver to pass is set and written here https://github.com/wongma7/csi-certify/blob/master/pkg/certify/driver/driver.go. Currently it's a copy of the hostPath CSI driver TestDriver from https://github.com/kubernetes/kubernetes/blob/master/test/e2e/storage/drivers/csi.go, but it could be anything. A "default" TestDriver implementation like https://github.com/kubernetes/kubernetes/pull/72836 that simply reads "static" information from a yaml will probably suffice for most CSI drivers. Otherwise if some "dynamic" configuration is needed then all a CSI driver author needs to do is modify the implementation of TestDriver in driver.go. If they wish to add custom tests, they can do that too by adding specs to csi_volumes.go or other files.

So the minimum API "surface" for me/CSI authors looks like: TestDriver, DriverInfo (field of TestDriver), PerTestConfig (param/return of TestDriver functions), TestVolume (param/return), DefineTestSuite, and the various testsuites.TestSuites. It's probably unnecessary to provide guarantees about things below, like TestDynamicProvisioning which is frequently changed and probably going to change again soon.

To run:
go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig

## fedora local cluster
sudo chown -R $USER:$USER /var/run/kubernetes/
sudo chown -R $USER:$USER /var/lib/kubelet
sudo chcon -R -t svirt_sandbox_file_t /var/lib/kubelet
