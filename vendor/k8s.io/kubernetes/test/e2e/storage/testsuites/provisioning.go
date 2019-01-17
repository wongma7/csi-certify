/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package testsuites

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"

	"k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/storage/testpatterns"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

// StorageClassTest represents parameters to be used by TestDynamicProvisioning
type StorageClassTest struct {
	Client             clientset.Interface
	Claim              *v1.PersistentVolumeClaim
	Class              *storage.StorageClass
	Name               string
	CloudProviders     []string
	Provisioner        string
	StorageClassName   string
	Parameters         map[string]string
	DelayBinding       bool
	ClaimSize          string
	ExpectedSize       string
	PvCheck            func(volume *v1.PersistentVolume) error
	NodeName           string
	SkipWriteReadCheck bool
	VolumeMode         *v1.PersistentVolumeMode
}

type provisioningTestSuite struct {
	tsInfo TestSuiteInfo
}

var _ TestSuite = &provisioningTestSuite{}

// InitProvisioningTestSuite returns provisioningTestSuite that implements TestSuite interface
func InitProvisioningTestSuite() TestSuite {
	return &provisioningTestSuite{
		tsInfo: TestSuiteInfo{
			name: "provisioning",
			testPatterns: []testpatterns.TestPattern{
				testpatterns.DefaultFsDynamicPV,
			},
		},
	}
}

func (p *provisioningTestSuite) getTestSuiteInfo() TestSuiteInfo {
	return p.tsInfo
}

func (p *provisioningTestSuite) defineTests(driver TestDriver, pattern testpatterns.TestPattern) {
	var (
		f             = framework.NewDefaultFramework("provisioning")
		dInfo         = driver.GetDriverInfo()
		config        *PerTestConfig
		driverCleanup func()
		testCase      *StorageClassTest
		pvc           *v1.PersistentVolumeClaim
		sc            *storage.StorageClass
	)

	init := func() {
		// Check preconditions.
		if pattern.VolType != testpatterns.DynamicPV {
			framework.Skipf("Suite %q does not support %v", p.tsInfo.name, pattern.VolType)
		}
		dDriver, ok := driver.(DynamicPVTestDriver)
		if !ok {
			framework.Skipf("Driver %s doesn't support %v -- skipping", dInfo.Name, pattern.VolType)
		}

		// Now do the more expensive test initialization.
		config, driverCleanup = driver.CreateDriver(f)
		claimSize := dDriver.GetClaimSize()
		sc = dDriver.GetDynamicProvisionStorageClass(config, "")
		if sc == nil {
			framework.Skipf("Driver %q does not define Dynamic Provision StorageClass - skipping", dInfo.Name)
		}
		pvc = getClaim(claimSize, config.Framework.Namespace.Name)
		pvc.Spec.StorageClassName = &sc.Name
		framework.Logf("In creating storage class object and pvc object for driver - sc: %v, pvc: %v", sc, pvc)
		testCase = &StorageClassTest{
			Client:       config.Framework.ClientSet,
			Claim:        pvc,
			Class:        sc,
			ClaimSize:    claimSize,
			ExpectedSize: claimSize,
			NodeName:     config.ClientNodeName,
		}
	}

	It("should provision storage with defaults", func() {
		init()
		defer driverCleanup()

		testCase.TestDynamicProvisioning()
	})

	It("should provision storage with mount options", func() {
		if dInfo.SupportedMountOption == nil {
			framework.Skipf("Driver %q does not define supported mount option - skipping", dInfo.Name)
		}

		init()
		defer driverCleanup()

		testCase.Class.MountOptions = dInfo.SupportedMountOption.Union(dInfo.RequiredMountOption).List()
		testCase.TestDynamicProvisioning()
	})

	It("should create and delete block persistent volumes", func() {
		if !dInfo.Capabilities[CapBlock] {
			framework.Skipf("Driver %q does not support BlockVolume - skipping", dInfo.Name)
		}

		init()
		defer driverCleanup()

		block := v1.PersistentVolumeBlock
		testCase.VolumeMode = &block
		testCase.SkipWriteReadCheck = true
		pvc.Spec.VolumeMode = &block
		testCase.TestDynamicProvisioning()
	})
}

// TestDynamicProvisioning tests dynamic provisioning with specified StorageClassTest
func (t StorageClassTest) TestDynamicProvisioning() *v1.PersistentVolume {
	client := t.Client
	claim := t.Claim
	class := t.Class

	var err error
	if class != nil {
		By("creating a StorageClass " + class.Name)
		class, err = client.StorageV1().StorageClasses().Create(class)
		Expect(err).NotTo(HaveOccurred())
		defer func() {
			framework.Logf("deleting storage class %s", class.Name)
			framework.ExpectNoError(client.StorageV1().StorageClasses().Delete(class.Name, nil))
		}()
	}

	By("creating a claim")
	claim, err = client.CoreV1().PersistentVolumeClaims(claim.Namespace).Create(claim)
	Expect(err).NotTo(HaveOccurred())
	defer func() {
		framework.Logf("deleting claim %q/%q", claim.Namespace, claim.Name)
		// typically this claim has already been deleted
		err = client.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(claim.Name, nil)
		if err != nil && !apierrs.IsNotFound(err) {
			framework.Failf("Error deleting claim %q. Error: %v", claim.Name, err)
		}
	}()
	err = framework.WaitForPersistentVolumeClaimPhase(v1.ClaimBound, client, claim.Namespace, claim.Name, framework.Poll, framework.ClaimProvisionTimeout)
	Expect(err).NotTo(HaveOccurred())

	By("checking the claim")
	// Get new copy of the claim
	claim, err = client.CoreV1().PersistentVolumeClaims(claim.Namespace).Get(claim.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Get the bound PV
	pv, err := client.CoreV1().PersistentVolumes().Get(claim.Spec.VolumeName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	// Check sizes
	expectedCapacity := resource.MustParse(t.ExpectedSize)
	pvCapacity := pv.Spec.Capacity[v1.ResourceName(v1.ResourceStorage)]
	Expect(pvCapacity.Value()).To(Equal(expectedCapacity.Value()), "pvCapacity is not equal to expectedCapacity")

	requestedCapacity := resource.MustParse(t.ClaimSize)
	claimCapacity := claim.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	Expect(claimCapacity.Value()).To(Equal(requestedCapacity.Value()), "claimCapacity is not equal to requestedCapacity")

	// Check PV properties
	By("checking the PV")
	expectedAccessModes := []v1.PersistentVolumeAccessMode{v1.ReadWriteOnce}
	Expect(pv.Spec.AccessModes).To(Equal(expectedAccessModes))
	Expect(pv.Spec.ClaimRef.Name).To(Equal(claim.ObjectMeta.Name))
	Expect(pv.Spec.ClaimRef.Namespace).To(Equal(claim.ObjectMeta.Namespace))
	if class == nil {
		Expect(pv.Spec.PersistentVolumeReclaimPolicy).To(Equal(v1.PersistentVolumeReclaimDelete))
	} else {
		Expect(pv.Spec.PersistentVolumeReclaimPolicy).To(Equal(*class.ReclaimPolicy))
		Expect(pv.Spec.MountOptions).To(Equal(class.MountOptions))
	}
	if t.VolumeMode != nil {
		Expect(pv.Spec.VolumeMode).NotTo(BeNil())
		Expect(*pv.Spec.VolumeMode).To(Equal(*t.VolumeMode))
	}

	// Run the checker
	if t.PvCheck != nil {
		err = t.PvCheck(pv)
		Expect(err).NotTo(HaveOccurred())
	}

	if !t.SkipWriteReadCheck {
		// We start two pods:
		// - The first writes 'hello word' to the /mnt/test (= the volume).
		// - The second one runs grep 'hello world' on /mnt/test.
		// If both succeed, Kubernetes actually allocated something that is
		// persistent across pods.
		By("checking the created volume is writable and has the PV's mount options")
		command := "echo 'hello world' > /mnt/test/data"
		// We give the first pod the secondary responsibility of checking the volume has
		// been mounted with the PV's mount options, if the PV was provisioned with any
		for _, option := range pv.Spec.MountOptions {
			// Get entry, get mount options at 6th word, replace brackets with commas
			command += fmt.Sprintf(" && ( mount | grep 'on /mnt/test' | awk '{print $6}' | sed 's/^(/,/; s/)$/,/' | grep -q ,%s, )", option)
		}
		command += " || (mount | grep 'on /mnt/test'; false)"
		runInPodWithVolume(client, claim.Namespace, claim.Name, t.NodeName, command)

		By("checking the created volume is readable and retains data")
		runInPodWithVolume(client, claim.Namespace, claim.Name, t.NodeName, "grep 'hello world' /mnt/test/data")
	}
	By(fmt.Sprintf("deleting claim %q/%q", claim.Namespace, claim.Name))
	framework.ExpectNoError(client.CoreV1().PersistentVolumeClaims(claim.Namespace).Delete(claim.Name, nil))

	// Wait for the PV to get deleted if reclaim policy is Delete. (If it's
	// Retain, there's no use waiting because the PV won't be auto-deleted and
	// it's expected for the caller to do it.) Technically, the first few delete
	// attempts may fail, as the volume is still attached to a node because
	// kubelet is slowly cleaning up the previous pod, however it should succeed
	// in a couple of minutes. Wait 20 minutes to recover from random cloud
	// hiccups.
	if pv.Spec.PersistentVolumeReclaimPolicy == v1.PersistentVolumeReclaimDelete {
		By(fmt.Sprintf("deleting the claim's PV %q", pv.Name))
		framework.ExpectNoError(framework.WaitForPersistentVolumeDeleted(client, pv.Name, 5*time.Second, 20*time.Minute))
	}

	return pv
}

// runInPodWithVolume runs a command in a pod with given claim mounted to /mnt directory.
func runInPodWithVolume(c clientset.Interface, ns, claimName, nodeName, command string) {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvc-volume-tester-",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    "volume-tester",
					Image:   imageutils.GetE2EImage(imageutils.BusyBox),
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", command},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "my-volume",
							MountPath: "/mnt/test",
						},
					},
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "my-volume",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: claimName,
							ReadOnly:  false,
						},
					},
				},
			},
		},
	}

	if len(nodeName) != 0 {
		pod.Spec.NodeName = nodeName
	}
	pod, err := c.CoreV1().Pods(ns).Create(pod)
	framework.ExpectNoError(err, "Failed to create pod: %v", err)
	defer func() {
		body, err := c.CoreV1().Pods(ns).GetLogs(pod.Name, &v1.PodLogOptions{}).Do().Raw()
		if err != nil {
			framework.Logf("Error getting logs for pod %s: %v", pod.Name, err)
		} else {
			framework.Logf("Pod %s has the following logs: %s", pod.Name, body)
		}
		framework.DeletePodOrFail(c, ns, pod.Name)
	}()
	framework.ExpectNoError(framework.WaitForPodSuccessInNamespaceSlow(c, pod.Name, pod.Namespace))
}
