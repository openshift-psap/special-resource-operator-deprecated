package specialresource

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	"github.com/openshift-psap/special-resource-operator/pkg/yamlutil"
	routev1 "github.com/openshift/api/route/v1"
	secv1 "github.com/openshift/api/security/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

var (
	manifests  = "/etc/kubernetes/special-resource/nvidia-gpu"
	kubeclient *kubernetes.Clientset
)

// AddKubeClient Add a native non-caching client for advanced CRUD operations
func AddKubeClient(cfg *rest.Config) error {
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	kubeclient = clientSet
	return nil
}

// Add3dpartyResourcesToScheme Adds 3rd party resources To the operator
func Add3dpartyResourcesToScheme(scheme *runtime.Scheme) error {

	if err := routev1.AddToScheme(scheme); err != nil {
		return err
	}

	if err := secv1.AddToScheme(scheme); err != nil {
		return err
	}

	return nil
}

func filePathWalkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func exitOnError(err error, msg string) {
	if err != nil {
		log.Error(err, msg)
		os.Exit(1)
	}
}

// ReconcileClusterResources Reconcile cluster resources
func ReconcileClusterResources(r *ReconcileSpecialResource) error {

	_, err := os.Stat(manifests)
	exitOnError(err, "Missing manifests dir: "+manifests)

	states, err := filePathWalkDir(manifests)
	exitOnError(err, "Cannot walk dir: "+manifests)

	for _, state := range states {

		log.Info("Executing", "State", state)
		namespacedYAML, err := ioutil.ReadFile(state)
		exitOnError(err, "Cannot read state file: "+state)

		if err := createFromYAML(namespacedYAML, r); err != nil {
			return err
		}
	}
	return nil
}

// EachResourceListItem Handle UnstructuredList
func EachResourceListItem(obj *unstructured.UnstructuredList, r *ReconcileSpecialResource,
	fn func(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error) error {

	field, ok := obj.Object["items"]
	if !ok {
		return errors.New("content is not a list")
	}
	items, ok := field.([]interface{})
	if !ok {
		return fmt.Errorf("content is not a list: %T", field)
	}
	for _, item := range items {
		child, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("items member is not an object: %T", child)
		}
		if err := fn(&unstructured.Unstructured{Object: child}, r); err != nil {
			return err
		}
	}
	return nil
}

// ReconcileResource prefix, CRUD, postfix
func ReconcileResource(obj interface{}, r *ReconcileSpecialResource) error {

	switch res := obj.(type) {
	case *unstructured.UnstructuredList:
		if err := EachResourceListItem(res, r, CRUD); err != nil {
			exitOnError(err, "CRUD exited non-zero")
		}
	case *unstructured.Unstructured:
		// Create Update Delete Patch resources
		if err := CRUD(res, r); err != nil {
			exitOnError(err, "CRUD exited non-zero")
		}
	default:
		log.Info("DEFAULT", "No idea what to do with: ", reflect.TypeOf(obj))
	}

	return nil
}

func createFromYAML(yamlFile []byte, r *ReconcileSpecialResource) error {

	scanner := yamlutil.NewYAMLScanner(yamlFile)

	for scanner.Scan() {

		yamlSpec := scanner.Bytes()
		obj := &unstructured.Unstructured{}
		jsonSpec, err := yaml.YAMLToJSON(yamlSpec)
		if err != nil {
			log.Error(err, "Could not convert yaml file to json")
			return err
		}
		obj.UnmarshalJSON(jsonSpec)

		// Callbacks before CRUD will update the manifests
		modified, err := prefixResourceCallback(obj, r)
		if err != nil {
			log.Error(err, "prefix callbacks exited non-zero")
			return err
		}

		ReconcileResource(modified, r)

		// Callbacks after CRUD will wait for ressource and check status
		_, err = postfixResourceCallback(obj, r)
		if err != nil {
			log.Error(err, "postfix callbacks exited non-zero")
			return err
		}

	}

	if err := scanner.Err(); err != nil {
		log.Error(err, "failed to scan manifest: ", err)
		return err
	}
	return nil
}

func updateResource(req *unstructured.Unstructured, found *unstructured.Unstructured) error {

	kind := found.GetKind()

	if kind == "SecurityContextConstraints" || kind == "Service" || kind == "ServiceMonitor" || kind == "Route" {

		version, fnd, err := unstructured.NestedString(found.Object, "metadata", "resourceVersion")
		checkNestedFields(fnd, err)

		if err := unstructured.SetNestedField(req.Object, version, "metadata", "resourceVersion"); err != nil {
			log.Error(err, "Couldn't update ResourceVersion")
			return err
		}

	}
	if kind == "Service" {
		clusterIP, fnd, err := unstructured.NestedString(found.Object, "spec", "clusterIP")
		checkNestedFields(fnd, err)

		if err := unstructured.SetNestedField(req.Object, clusterIP, "spec", "clusterIP"); err != nil {
			log.Error(err, "Couldn't update clusterIP")
			return err
		}
		return nil
	}
	return nil
}

// CRUD Create Update Delete Resource
func CRUD(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error {

	obj.SetNamespace(r.specialresource.Namespace)

	logger := log.WithValues("Kind", obj.GetKind(), "Namespace", obj.GetNamespace(), "Name", obj.GetName())
	found := obj.DeepCopy()

	if err := controllerutil.SetControllerReference(r.specialresource, obj, r.scheme); err != nil {
		log.Error(err, "Failed to set controller reference")
		return err
	}

	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, found)

	if apierrors.IsNotFound(err) {
		logger.Info("Not found, creating")
		if err := r.client.Create(context.TODO(), obj); err != nil {
			return fmt.Errorf("Couldn't Create (%v)", err)
		}
		return nil
	}
	if err == nil && obj.GetKind() != "ServiceAccount" && obj.GetKind() != "Pod" {

		logger.Info("Found, updating")
		required := obj.DeepCopy()

		if err := updateResource(required, found); err != nil {
			log.Error(err, "Couldn't Update Resource")
			return err
		}
		//required.ResourceVersion = found.ResourceVersion

		if err := r.client.Update(context.TODO(), required); err != nil {
			return fmt.Errorf("Couldn't Update (%v)", err)
		}
		return nil
	}

	if err != nil {
		logger.Error(err, "UNEXPECTED ERROR")
	}

	return nil
}
