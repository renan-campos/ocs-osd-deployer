/*


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

package controllers

import (
	"context"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	promv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	ocsv1 "github.com/openshift/ocs-operator/pkg/apis"
	v1 "github.com/openshift/ocs-osd-deployer/api/v1alpha1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

const (
	testPrimaryNamespace             = "primary"
	testSecondaryNamespace           = "secondary"
	testAddonParamsSecretName        = "test-addon-secret"
	testPagerdutySecretName          = "test-pagerduty-secret"
	testDeadMansSnitchSecretName     = "test-deadmanssnitch-secret"
	testAddonConfigMapName           = "test-addon-configmap"
	testAddonConfigMapDeleteLabelKey = "test-addon-configmap-delete-label-key"
	testSubscriptionName             = "test-subscription"
	testDeployerCSVName              = "ocs-osd-deployer.x.y.z"
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "config", "crd", "bases"),
			filepath.Join("..", "shim", "crds"),
		},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = ocsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = promv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = v1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = opv1a1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&ManagedOCSReconciler{
		Client:                       k8sManager.GetClient(),
		UnrestrictedClient:           k8sManager.GetClient(),
		Log:                          ctrl.Log.WithName("controllers").WithName("ManagedOCS"),
		Scheme:                       scheme.Scheme,
		AddonParamSecretName:         testAddonParamsSecretName,
		AddonConfigMapName:           testAddonConfigMapName,
		AddonConfigMapDeleteLabelKey: testAddonConfigMapDeleteLabelKey,
		DeployerSubscriptionName:     testSubscriptionName,
		PagerdutySecretName:          testPagerdutySecretName,
		DeadMansSnitchSecretName:     testDeadMansSnitchSecretName,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	// Client to be use by the test code, using a non cached client
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	ctx := context.Background()

	// Create the primary namespace to be used by some of the tests
	primaryNS := &corev1.Namespace{}
	primaryNS.Name = testPrimaryNamespace
	Expect(k8sClient.Create(ctx, primaryNS)).Should(Succeed())

	// Create a secondary namespace to be used by some of the tests
	secondaryNS := &corev1.Namespace{}
	secondaryNS.Name = testSecondaryNamespace
	Expect(k8sClient.Create(ctx, secondaryNS)).Should(Succeed())

	// Create a mock subscription
	deployerSub := &opv1a1.Subscription{}
	deployerSub.Name = testSubscriptionName
	deployerSub.Namespace = testPrimaryNamespace
	deployerSub.Spec = &opv1a1.SubscriptionSpec{}
	Expect(k8sClient.Create(ctx, deployerSub)).ShouldNot(HaveOccurred())

	// create a mock deplyer CSV
	deployerCSV := &opv1a1.ClusterServiceVersion{}
	deployerCSV.Name = testDeployerCSVName
	deployerCSV.Namespace = testPrimaryNamespace
	deployerCSV.Spec.InstallStrategy.StrategyName = "test-strategy"
	deployerCSV.Spec.InstallStrategy.StrategySpec.DeploymentSpecs = []opv1a1.StrategyDeploymentSpec{}
	Expect(k8sClient.Create(ctx, deployerCSV)).ShouldNot(HaveOccurred())

	// Create the ManagedOCS resource
	managedOCS := &v1.ManagedOCS{}
	managedOCS.Name = managedOCSName
	managedOCS.Namespace = testPrimaryNamespace
	Expect(k8sClient.Create(ctx, managedOCS)).ShouldNot(HaveOccurred())

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
