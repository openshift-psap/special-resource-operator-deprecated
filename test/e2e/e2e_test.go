// Copyright 2018 The Operator-SDK Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	goctx "context"
	"io/ioutil"
	"testing"
	"time"

	apis "github.com/openshift-psap/special-resource-operator/pkg/apis"
	operator "github.com/openshift-psap/special-resource-operator/pkg/apis/sro/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/operator-framework/operator-sdk/pkg/test"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

//func GetLogs(kubeClient kubernetes.Interface, namespace string, podName, containerName string) (string, error) {
//	logs, err := kubeClient.CoreV1().RESTClient().Get().
//		Resource("pods").
//		Namespace(namespace).
//		Name(podName).SubResource("log").
//		Param("container", containerName).
//		Do().
//		Raw()
//	if err != nil {
//		return "", err
//	}
//	return string(logs), err
//}

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 30
	opName               = "gpu"
	opNamespace          = "openshift-sro"
	opImage              = "quay.io/openshift-psap/speic:v4.2"
	//opImage = "registry.svc.ci.openshift.org/openshift/node-feature-discovery-container:v4.2"
)

func TestSpecialResourceAddScheme(t *testing.T) {
	sroList := &operator.SpecialResourceList{}

	err := framework.AddToFrameworkScheme(apis.AddToScheme, sroList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}
}

func TestSpecialResource(t *testing.T) {
	ctx := framework.NewTestCtx(t)

	defer ctx.Cleanup()

	//err := ctx.InitializeClusterResources("/home/openshift-psap/go/src/github.com/openshift-psap/special-resource-operator/manifests/operator-init.yaml")
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")

	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}

	f := framework.Global
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "special-resource-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}

	if err = driverContainerBase(t, f, ctx); err != nil {
		t.Fatal(err)
	}
}

func createClusterRoleBinding(t *testing.T, namespace string, ctx *framework.TestCtx) error {
	// operator-sdk test cannot deploy clusterrolebinding
	obj := &rbacv1.ClusterRoleBinding{}

	namespacedYAML, err := ioutil.ReadFile("manifests/0400_cluster_role_binding.yaml")
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme,
		scheme.Scheme)

	_, _, err = s.Decode(namespacedYAML, nil, obj)

	obj.SetNamespace(namespace)

	obj.Subjects[0].Namespace = namespace

	for _, subject := range obj.Subjects {
		if subject.Kind == "ServiceAccount" {
			subject.Namespace = namespace
		}
	}

	err = test.Global.Client.Create(goctx.TODO(), obj,
		&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})

	if apierrors.IsAlreadyExists(err) {
		t.Errorf("ClusterRoleBinding already exists: %s", obj.Name)
	}

	return err
}

func createSpecialResource(t *testing.T, ctx *framework.TestCtx, namespace string, name string) error {

	return nil
}

func driverContainerBase(t *testing.T, f *framework.Framework, ctx *framework.TestCtx) error {

	// Create DriverContainerBase Manifests
	createSpecialResource(t, ctx, "driver-container-base", "driver-container-base")

	return nil
}

func WaitForDaemonSet(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, replicas int, retryInterval, timeout time.Duration) error {
	return waitForDaemonSet(t, kubeclient, namespace, name, replicas, retryInterval, timeout, false)
}

func waitForDaemonSet(t *testing.T, kubeclient kubernetes.Interface, namespace, name string, replicas int, retryInterval, timeout time.Duration, isOperator bool) error {
	if isOperator && test.Global.LocalOperator {
		t.Log("Operator is running locally; skip waitForDaemonSet")
		return nil
	}
	err := wait.Poll(retryInterval, timeout, func() (done bool, err error) {
		daemonset, err := kubeclient.AppsV1().DaemonSets(namespace).Get(name, metav1.GetOptions{IncludeUninitialized: true})
		if err != nil {
			if apierrors.IsNotFound(err) {
				t.Logf("Waiting for availability of %s/%s DaemonSet\n", namespace, name)
				return false, nil
			}
			return false, err
		}

		if int(daemonset.Status.NumberUnavailable) == 0 {
			return true, nil
		}
		t.Logf("Waiting for full availability of %s/%s DaemonSet NumberUnavailable (%d/%d)\n", namespace, name, daemonset.Status.NumberUnavailable, 0)
		return false, nil
	})
	if err != nil {
		return err
	}
	t.Logf("DaemonSet NumberUnavailable (%d/%d)\n", 0, 0)
	return nil
}
