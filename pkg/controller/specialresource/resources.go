package specialresource

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	errs "github.com/pkg/errors"

	monitoringV1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/openshift-psap/special-resource-operator/pkg/yamlutil"
	buildV1 "github.com/openshift/api/build/v1"
	imageV1 "github.com/openshift/api/image/v1"
	routev1 "github.com/openshift/api/route/v1"
	secv1 "github.com/openshift/api/security/v1"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

type nodes struct {
	list  *unstructured.UnstructuredList
	count int64
}

var (
	manifests  = "/etc/kubernetes/special-resource/nvidia-gpu"
	kubeclient *kubernetes.Clientset
	node       = nodes{
		list:  &unstructured.UnstructuredList{},
		count: 0xDEADBEEF,
	}
	operatingSystem = ""
	kernelVersion   = ""
	clusterVersion  = ""
	updateVendor    = ""
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
	if err := buildV1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := imageV1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := monitoringV1.AddToScheme(scheme); err != nil {
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

func exitOnError(err error) {
	if err != nil {
		log.Info("Exiting On Error", "error", err)
		os.Exit(1)
	}
}

func cacheNodes(r *ReconcileSpecialResource, force bool) (*unstructured.UnstructuredList, error) {

	// The initial list is what we're working with
	// a SharedInformer will update the list of nodes if
	// more nodes join the cluster.
	cached := int64(len(node.list.Items))
	if cached == node.count && !force {
		return node.list, nil
	}

	node.list.SetAPIVersion("v1")
	node.list.SetKind("NodeList")

	opts := &client.ListOptions{}
	opts.SetLabelSelector("feature.node.kubernetes.io/pci-10de.present=true")

	err := r.client.List(context.TODO(), opts, node.list)
	if err != nil {
		return nil, errors.Wrap(err, "Client cannot get NodeList")
	}

	return node.list, err
}

func getSROstatesCM(r *ReconcileSpecialResource) (map[string]interface{}, []string, error) {

	log.Info("Looking for ConfigMap special-resource-operator-states")
	cm := &unstructured.Unstructured{}

	cm.SetAPIVersion("v1")
	cm.SetKind("ConfigMap")

	ns := r.specialresource.GetNamespace()
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: "special-resource-operator-states"}, cm)

	if apierrors.IsNotFound(err) {
		log.Info("ConfigMap special-resource-states not found, see README and create the states")
		return nil, nil, nil
	}

	manifests, found, err := unstructured.NestedMap(cm.Object, "data")
	checkNestedFields(found, err)

	states := make([]string, 0, len(manifests))
	for key := range manifests {
		states = append(states, key)
	}

	sort.Strings(states)

	return manifests, states, nil

}

// ReconcileClusterResources Reconcile cluster resources
func ReconcileClusterResources(r *ReconcileSpecialResource) error {

	manifests, states, err := getSROstatesCM(r)

	node.list, err = cacheNodes(r, false)
	exitOnError(errs.Wrap(err, "Failed to cache Nodes"))

	operatingSystem, err = getOperatingSystem()
	exitOnError(errs.Wrap(err, "Failed to get operating system"))

	kernelVersion, err = getKernelVersion()
	exitOnError(errs.Wrap(err, "Failed to get kernel version"))

	clusterVersion, err = getClusterVersion()
	exitOnError(errs.Wrap(err, "Failed to get cluster version"))

	for _, state := range states {

		log.Info("Executing", "State", state)
		namespacedYAML := []byte(manifests[state].(string))
		if err := createFromYAML(namespacedYAML, r); err != nil {
			return errs.Wrap(err, "Failed to create resources")
		}
	}

	return nil
}

func createFromYAML(yamlFile []byte, r *ReconcileSpecialResource) error {

	namespace := r.specialresource.Namespace
	scanner := yamlutil.NewYAMLScanner(yamlFile)

	for scanner.Scan() {

		yamlSpec := scanner.Bytes()
		obj := &unstructured.Unstructured{}
		jsonSpec, err := yaml.YAMLToJSON(yamlSpec)
		if err != nil {
			return errs.Wrap(err, "Could not convert yaml file to json")
		}

		err = injectRuntimeInformation(&jsonSpec)
		exitOnError(errs.Wrap(err, "Cannot inject runtime information"))

		err = obj.UnmarshalJSON(jsonSpec)
		exitOnError(errs.Wrap(err, "Cannot unmarshall json spec, check your manifests"))

		// We are only building a driver-container if we cannot pull the image
		if obj.GetKind() == "BuildConfig" && updateVendor == "" {
			log.Info("Skpping building driver-container", "Name", obj.GetName())
			return nil
		}

		obj.SetNamespace(namespace)

		// Callbacks before CRUD will update the manifests
		if err := beforeCRUDhooks(obj, r); err != nil {
			return errs.Wrap(err, "Before CRUD hooks failed")
		}
		// Create Update Delete Patch resources
		err = CRUD(obj, r)
		exitOnError(errs.Wrap(err, "CRUD exited non-zero"))

		// Callbacks after CRUD will wait for ressource and check status
		if err := afterCRUDhooks(obj, r); err != nil {
			return errs.Wrap(err, "After CRUD hooks failed")
		}

	}

	if err := scanner.Err(); err != nil {
		return errs.Wrap(err, "Failed to scan manifest")
	}
	return nil
}

// Some resources need an updated resourceversion, during updates
func needToUpdateResourceVersion(kind string) bool {

	if kind == "SecurityContextConstraints" ||
		kind == "Service" ||
		kind == "ServiceMonitor" ||
		kind == "Route" ||
		kind == "BuildConfig" ||
		kind == "ImageStream" ||
		kind == "PrometheusRule" {
		return true
	}
	return false
}

func updateResource(req *unstructured.Unstructured, found *unstructured.Unstructured) error {

	kind := found.GetKind()

	if needToUpdateResourceVersion(kind) {
		version, fnd, err := unstructured.NestedString(found.Object, "metadata", "resourceVersion")
		checkNestedFields(fnd, err)

		if err := unstructured.SetNestedField(req.Object, version, "metadata", "resourceVersion"); err != nil {
			return errs.Wrap(err, "Couldn't update ResourceVersion")
		}

	}
	if kind == "Service" {
		clusterIP, fnd, err := unstructured.NestedString(found.Object, "spec", "clusterIP")
		checkNestedFields(fnd, err)

		if err := unstructured.SetNestedField(req.Object, clusterIP, "spec", "clusterIP"); err != nil {
			return errs.Wrap(err, "Couldn't update clusterIP")
		}
		return nil
	}
	return nil
}

// CRUD Create Update Delete Resource
func CRUD(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error {

	logger := log.WithValues("Kind", obj.GetKind(), "Namespace", obj.GetNamespace(), "Name", obj.GetName())
	found := obj.DeepCopy()

	if err := controllerutil.SetControllerReference(r.specialresource, obj, r.scheme); err != nil {
		return errs.Wrap(err, "Failed to set controller reference")
	}

	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}, found)

	if apierrors.IsNotFound(err) {
		logger.Info("Not found, creating")
		if err := r.client.Create(context.TODO(), obj); err != nil {
			return errs.Wrap(err, "Couldn't Create Resource")
		}
		return nil
	}

	if apierrors.IsForbidden(err) {
		return errs.Wrap(err, "Forbidden check Role, ClusterRole and Bindings for operator")
	}

	if err != nil {
		return errs.Wrap(err, "Unexpected error")
	}

	// ServiceAccounts cannot be updated, maybe delete and create?
	if obj.GetKind() == "ServiceAccount" {
		logger.Info("TODO: Found, not updating")
		return nil
	}

	logger.Info("Found, updating")
	required := obj.DeepCopy()

	// required.ResourceVersion = found.ResourceVersion
	if err := updateResource(required, found); err != nil {
		return errs.Wrap(err, "Couldn't Update ResourceVersion")
	}

	// BuildConfig are currently not triggered by an update need to delete first
	if obj.GetKind() == "BuildConfig" {

		labels := obj.GetLabels()
		if vendor, ok := labels["specialresource.openshift.io/driver-container-vendor"]; ok && vendor == updateVendor {
			err := r.client.Delete(context.TODO(), obj)
			exitOnError(errs.Wrap(err, "Couldn't Delete BuildConfig"))
		}
		logger.Info("TODO: BuildConfig not triggered due to Update, reconciling")
	}

	if err := r.client.Update(context.TODO(), required); err != nil {
		return errs.Wrap(err, "Couldn't Update Resource")
	}

	return nil
}
