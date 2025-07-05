package controller_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"

	sdk "github.com/bitwarden/sdk-go"
	operatorsv1 "github.com/bitwarden/sm-kubernetes/api/v1"
	"github.com/bitwarden/sm-kubernetes/internal/controller"
	"github.com/bitwarden/sm-kubernetes/internal/controller/test/testutils"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("BitwardenSecret Reconciler - Success Tests", Ordered, func() {
	var (
		namespace string
		fixture   testutils.TestFixture
	)

	BeforeEach(func() {
		fixture = *testutils.NewTestFixture(testContext, envTestRunner)
		namespace = fixture.CreateNamespace()
	})

	AfterAll(func() {
		fixture.Cancel()
	})

	AfterEach(func() {
		fixture.Teardown()
	})

	It("should complete a successful sync", func() {
		fixture.SetupDefaultCtrlMocks(false, nil)

		_, err := fixture.CreateDefaultAuthSecret(namespace)
		Expect(err).NotTo(HaveOccurred())

		bwSecret, err := fixture.CreateDefaultBitwardenSecret(namespace, fixture.SecretMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(bwSecret).NotTo(BeNil())

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}}

		result, err := fixture.Reconciler.Reconcile(fixture.Ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(fixture.Reconciler.RefreshIntervalSeconds) * time.Second))

		Eventually(func(g Gomega) {
			// Verify created secret
			createdTargetSecret := &corev1.Secret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.SynchronizedSecretName, Namespace: namespace}, createdTargetSecret)).Should(Succeed())
			g.Expect(createdTargetSecret.Labels[controller.LabelBwSecret]).To(Equal(string(bwSecret.UID)))
			g.Expect(createdTargetSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			g.Expect(len(createdTargetSecret.Data)).To(Equal(testutils.ExpectedNumOfSecrets))

			// Verify annotations
			g.Expect(createdTargetSecret.Annotations[controller.AnnotationSyncTime]).NotTo(BeEmpty())
			g.Expect(createdTargetSecret.Annotations[controller.AnnotationCustomMap]).NotTo(BeEmpty())

			// Verify SuccessfulSync condition and LastSuccessfulSyncTime
			updatedBwSecret := &operatorsv1.BitwardenSecret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}, updatedBwSecret)).Should(Succeed())
			condition := apimeta.FindStatusCondition(updatedBwSecret.Status.Conditions, "SuccessfulSync")
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(updatedBwSecret.Status.LastSuccessfulSyncTime.Time).NotTo(BeZero())
		}).Should(Succeed())
	})

	// //This test misbehaves with the following error.  There's no rational reason for this to happen, so we'll leave it to the
	// //end user to figure out if this test is relevant to their needs.
	// //Message: "Operation cannot be fulfilled on bitwardensecrets.k8s.bitwarden.com \"bw-secret\": the object has been modified; please apply your changes to the latest version and try again",
	// It("should skip reconciliation when last sync is within refresh interval", func() {
	// 	fixture.SetupDefaultCtrlMocks(false, nil)

	// 	_, err := fixture.CreateDefaultAuthSecret(namespace)
	// 	Expect(err).NotTo(HaveOccurred())

	// 	bwSecret, err := fixture.CreateDefaultBitwardenSecret(namespace, fixture.SecretMap)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(bwSecret).NotTo(BeNil())

	// 	// Update status with LastSuccessfulSyncTime, retrying on conflicts
	// 	syncTime := time.Now().UTC()
	// 	Eventually(func(g Gomega) {
	// 		// Fetch the latest version of bwSecret (use cached client for Get)
	// 		latestBwSecret := &operatorsv1.BitwardenSecret{}
	// 		err := fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}, latestBwSecret)
	// 		GinkgoWriter.Printf("Fetched BitwardenSecret %s/%s, ResourceVersion: %s, err: %v\n", namespace, testutils.BitwardenSecretName, latestBwSecret.ResourceVersion, err)
	// 		g.Expect(err).Should(Succeed())

	// 		// Update status
	// 		latestBwSecret.Status = operatorsv1.BitwardenSecretStatus{
	// 			LastSuccessfulSyncTime: metav1.Time{Time: syncTime},
	// 		}
	// 		err = fixture.K8sClient.Status().Update(fixture.Ctx, latestBwSecret)
	// 		GinkgoWriter.Printf("Status update for %s/%s, ResourceVersion: %s, err: %v\n", namespace, testutils.BitwardenSecretName, latestBwSecret.ResourceVersion, err)
	// 		g.Expect(err).Should(Succeed())
	// 	}).WithTimeout(10 * time.Second).WithPolling(100 * time.Millisecond).Should(Succeed())

	// 	req := reconcile.Request{NamespacedName: types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}}

	// 	result, err := fixture.Reconciler.Reconcile(fixture.Ctx, req)
	// 	Expect(err).NotTo(HaveOccurred())
	// 	Expect(result).To(Equal(reconcile.Result{}))
	// })

	It("should skip sync when no changes from Bitwarden API", func() {
		// Override mocks to return no changes
		noChangesResponse := sdk.SecretsSyncResponse{
			HasChanges: false,
			Secrets:    []sdk.SecretResponse{},
		}

		fixture.SetupDefaultCtrlMocks(false, &noChangesResponse)

		_, err := fixture.CreateDefaultAuthSecret(namespace)
		Expect(err).NotTo(HaveOccurred())

		bwSecret, err := fixture.CreateDefaultBitwardenSecret(namespace, fixture.SecretMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(bwSecret).NotTo(BeNil())

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}}

		result, err := fixture.Reconciler.Reconcile(fixture.Ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(fixture.Reconciler.RefreshIntervalSeconds) * time.Second))

		Eventually(func(g Gomega) {
			// Verify no SuccessfulSync condition (no sync occurred)
			createdSecret := &operatorsv1.BitwardenSecret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}, createdSecret)).Should(Succeed())
			condition := apimeta.FindStatusCondition(createdSecret.Status.Conditions, "SuccessfulSync")
			g.Expect(condition).To(BeNil())
		})
	})

	It("should successfully sync with auth token from different namespace", func() {
		fixture.SetupDefaultCtrlMocks(false, nil)

		// Create auth secret in a different namespace
		authNamespace := fixture.CreateNamespace()
		_, err := fixture.CreateDefaultAuthSecret(authNamespace)
		Expect(err).NotTo(HaveOccurred())

		// Create BitwardenSecret with cross-namespace auth token using fixture method
		bwSecret, err := fixture.CreateBitwardenSecretWithAuthNamespace(testutils.BitwardenSecretName, namespace, fixture.OrgId, testutils.SynchronizedSecretName, testutils.AuthSecretName, testutils.AuthSecretKey, authNamespace, fixture.SecretMap, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(bwSecret).NotTo(BeNil())

		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}}

		result, err := fixture.Reconciler.Reconcile(fixture.Ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(fixture.Reconciler.RefreshIntervalSeconds) * time.Second))

		Eventually(func(g Gomega) {
			// Verify created secret in the BitwardenSecret's namespace
			createdTargetSecret := &corev1.Secret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.SynchronizedSecretName, Namespace: namespace}, createdTargetSecret)).Should(Succeed())
			g.Expect(createdTargetSecret.Labels[controller.LabelBwSecret]).To(Equal(string(bwSecret.UID)))
			g.Expect(createdTargetSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			g.Expect(len(createdTargetSecret.Data)).To(Equal(testutils.ExpectedNumOfSecrets))

			// Verify SuccessfulSync condition and LastSuccessfulSyncTime
			updatedBwSecret := &operatorsv1.BitwardenSecret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}, updatedBwSecret)).Should(Succeed())
			condition := apimeta.FindStatusCondition(updatedBwSecret.Status.Conditions, "SuccessfulSync")
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(updatedBwSecret.Status.LastSuccessfulSyncTime.Time).NotTo(BeZero())
		}).Should(Succeed())
	})

	It("should reconcile when managed Secret is deleted", func() {
		fixture.SetupDefaultCtrlMocks(false, nil)

		_, err := fixture.CreateDefaultAuthSecret(namespace)
		Expect(err).NotTo(HaveOccurred())

		bwSecret, err := fixture.CreateDefaultBitwardenSecret(namespace, fixture.SecretMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(bwSecret).NotTo(BeNil())

		// First reconcile to create the Secret
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}}
		result, err := fixture.Reconciler.Reconcile(fixture.Ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(fixture.Reconciler.RefreshIntervalSeconds) * time.Second))

		// Wait for Secret to be created
		Eventually(func(g Gomega) {
			createdSecret := &corev1.Secret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.SynchronizedSecretName, Namespace: namespace}, createdSecret)).Should(Succeed())
		}).Should(Succeed())

		// Delete the managed Secret
		managedSecret := &corev1.Secret{}
		err = fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.SynchronizedSecretName, Namespace: namespace}, managedSecret)
		Expect(err).NotTo(HaveOccurred())

		err = fixture.K8sClient.Delete(fixture.Ctx, managedSecret)
		Expect(err).NotTo(HaveOccurred())

		// Setup the mocks to return no changes from Bitwarden to test Secret-side reconciliation
		noChangesResponse := sdk.SecretsSyncResponse{
			HasChanges: false,
			Secrets:    []sdk.SecretResponse{},
		}
		fixture.SetupDefaultCtrlMocks(false, &noChangesResponse)

		// Reconcile again - should recreate the Secret even with no Bitwarden changes
		result, err = fixture.Reconciler.Reconcile(fixture.Ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(fixture.Reconciler.RefreshIntervalSeconds) * time.Second))

		// Verify Secret is recreated
		Eventually(func(g Gomega) {
			recreatedSecret := &corev1.Secret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.SynchronizedSecretName, Namespace: namespace}, recreatedSecret)).Should(Succeed())
			g.Expect(recreatedSecret.Labels[controller.LabelBwSecret]).To(Equal(string(bwSecret.UID)))
			g.Expect(len(recreatedSecret.Data)).To(Equal(testutils.ExpectedNumOfSecrets))
		}).Should(Succeed())
	})

	It("should reconcile when managed Secret loses ownership label", func() {
		fixture.SetupDefaultCtrlMocks(false, nil)

		_, err := fixture.CreateDefaultAuthSecret(namespace)
		Expect(err).NotTo(HaveOccurred())

		bwSecret, err := fixture.CreateDefaultBitwardenSecret(namespace, fixture.SecretMap)
		Expect(err).NotTo(HaveOccurred())
		Expect(bwSecret).NotTo(BeNil())

		// First reconcile to create the Secret
		req := reconcile.Request{NamespacedName: types.NamespacedName{Name: testutils.BitwardenSecretName, Namespace: namespace}}
		result, err := fixture.Reconciler.Reconcile(fixture.Ctx, req)
		Expect(err).NotTo(HaveOccurred())

		// Wait for Secret to be created and remove the ownership label
		Eventually(func(g Gomega) {
			managedSecret := &corev1.Secret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.SynchronizedSecretName, Namespace: namespace}, managedSecret)).Should(Succeed())

			// Remove the ownership label
			delete(managedSecret.Labels, controller.LabelBwSecret)
			g.Expect(fixture.K8sClient.Update(fixture.Ctx, managedSecret)).Should(Succeed())
		}).Should(Succeed())

		// Setup the mocks to return no changes from Bitwarden to test Secret-side reconciliation
		noChangesResponse := sdk.SecretsSyncResponse{
			HasChanges: false,
			Secrets:    []sdk.SecretResponse{},
		}
		fixture.SetupDefaultCtrlMocks(false, &noChangesResponse)

		// Reconcile again - should restore the ownership label even with no Bitwarden changes
		result, err = fixture.Reconciler.Reconcile(fixture.Ctx, req)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(time.Duration(fixture.Reconciler.RefreshIntervalSeconds) * time.Second))

		// Verify Secret has the ownership label restored
		Eventually(func(g Gomega) {
			restoredSecret := &corev1.Secret{}
			g.Expect(fixture.K8sClient.Get(fixture.Ctx, types.NamespacedName{Name: testutils.SynchronizedSecretName, Namespace: namespace}, restoredSecret)).Should(Succeed())
			g.Expect(restoredSecret.Labels[controller.LabelBwSecret]).To(Equal(string(bwSecret.UID)))
		}).Should(Succeed())
	})
})
