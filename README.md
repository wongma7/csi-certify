go test -v ./cmd/... -ginkgo.v -ginkgo.progress --kubeconfig=/var/run/kubernetes/admin.kubeconfig

> make sure hostPath /var/lib/kubelet/* is r/w by containers
