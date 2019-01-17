go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig

## fedora local cluster
sudo chown -R $USER:$USER /var/run/kubernetes/
sudo chown -R $USER:$USER /var/lib/kubelet
sudo chcon -R -t svirt_sandbox_file_t /var/lib/kubelet
