package e2e

import (
	goctx "context"
	apis "github.com/aerogear/unifiedpush-operator/pkg/apis"
	pushv1alpha1 "github.com/aerogear/unifiedpush-operator/pkg/apis/push/v1alpha1"
	dcv1 "github.com/openshift/client-go/apps/clientset/versioned/typed/apps/v1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"testing"
	"time"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 300
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

func TestUnifiedpush(t *testing.T) {
	unifiedpushList := &pushv1alpha1.UnifiedPushServerList{}
	if err := framework.AddToFrameworkScheme(apis.AddToScheme, unifiedpushList); err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
	t.Run("unifiedpush-e2e", UnifiedpushTest)
}

func UnifiedpushTest(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()

	f := framework.Global
	namespace, err := ctx.GetNamespace()
	unifiedPushServerName := "test-unifiedpushserver"
	pushServerTestCR := &pushv1alpha1.UnifiedPushServer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "UnifiedPushServer",
			APIVersion: "push.aerogear.org/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      unifiedPushServerName,
			Namespace: namespace,
		},
	}

	if err != nil {
		t.Fatalf("failed to get namespace: %v", err)
	}

	if err := initializePushResources(t, f, ctx, namespace); err != nil {
		t.Fatal(err)
	}

	// Create UPS CR
	if err := createPushServerCustomResource(t, f, ctx, pushServerTestCR); err != nil {
		t.Fatal(err)
	}

	// Additional client needed for retrieving deploymentConfigs
	dcV1Client, err := dcv1.NewForConfig(f.KubeConfig)
	if err != nil {
		t.Fatalf("Failed to initialize DeploymentConfig Client: %v", err)
	}

	// Ensure UPS was deployed successfully
	if err := waitForDeploymentConfig(t, *dcV1Client, namespace, unifiedPushServerName, 1); err != nil {
		t.Fatal(err)
	}
	t.Log("UPS deployment was successful")

	// Delete UPS CR
	if err := deletePushServerCustomResource(t, f, ctx, pushServerTestCR); err != nil {
		t.Fatal(err)
	}

	// Ensure UPS was deleted successfully
	if err := waitForDeploymentConfig(t, *dcV1Client, namespace, unifiedPushServerName, 0); err != nil {
		t.Fatal(err)
	}
	t.Log("UPS was deleted successfully")

}

func initializePushResources(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, namespace string) error {
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	})

	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Successfully initialized cluster resources")

	if err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "unifiedpush-operator", 1, retryInterval, timeout); err != nil {
		t.Fatal(err)
	}

	t.Log("Unified Push Operator successfully deployed")

	return nil
}

func createPushServerCustomResource(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, testCr *pushv1alpha1.UnifiedPushServer) error {

	err := f.Client.Create(goctx.TODO(), testCr, &framework.CleanupOptions{
		TestContext:   ctx,
		Timeout:       cleanupTimeout,
		RetryInterval: cleanupRetryInterval,
	})
	if err != nil {
		return err
	}
	t.Log("Successfully created UnifiedPushServer Custom Resource")

	return nil
}

func deletePushServerCustomResource(t *testing.T, f *framework.Framework, ctx *framework.TestCtx, testCr *pushv1alpha1.UnifiedPushServer) error {

	err := f.Client.Delete(goctx.TODO(), testCr)
	if err != nil {
		return err
	}
	t.Log("Successfully deleted UnifiedPushServer Custom Resource")

	return nil
}

// Helper function for checking whether specified DeploymentConfig has a certain number of available replicas
// Copied & edited from https://github.com/operator-framework/operator-sdk/blob/f6d83791dd8880f0e33d549343642aabadc9d3a0/pkg/test/e2eutil/wait_util.go#L46
func waitForDeploymentConfig(t *testing.T, dcV1Client dcv1.AppsV1Client, namespace, name string, replicas int) error {
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		dc, err := dcV1Client.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{IncludeUninitialized: true})

		if err != nil {
			if apierrors.IsNotFound(err) && replicas == 0 {
				return true, nil
			}
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s Deployment Config\n", name)
				return false, nil
			}
			return false, err
		}

		if int(dc.Status.AvailableReplicas) == replicas {
			return true, nil
		}
		t.Logf("Waiting for full availability of %s Deployment Config (%d/%d)\n", name, dc.Status.AvailableReplicas, replicas)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("Deployment Config has now requested number of replicas: (%d/%d)\n", replicas, replicas)
	return nil
}
