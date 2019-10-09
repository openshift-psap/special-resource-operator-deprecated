package specialresource

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/adler32"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func prefixNVIDIAdriverDaemonset(obj *unstructured.Unstructured, r *ReconcileSpecialResource) (interface{}, error) {

	containers, found, err := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "containers")
	checkNestedFields(found, err)

	kernelVersion := kernelFullVersion(r)

	for _, container := range containers {
		switch container := container.(type) {
		case map[string]interface{}:
			if container["name"] == "nvidia-driver-ctr" {
				img, found, err := unstructured.NestedString(container, "image")
				checkNestedFields(found, err)
				img = strings.Replace(img, "KERNEL_FULL_VERSION", kernelVersion, -1)
				err = unstructured.SetNestedField(container, img, "image")
				checkNestedFields(true, err)
			}
		default:
			panic(fmt.Errorf("cannot extract name,image from %T", container))
		}
	}

	err = unstructured.SetNestedSlice(obj.Object, containers,
		"spec", "template", "spec", "containers")
	checkNestedFields(true, err)

	err = unstructured.SetNestedField(obj.Object, kernelVersion,
		"spec", "template", "spec", "nodeSelector", "feature.node.kubernetes.io/kernel-version.full")
	checkNestedFields(true, err)

	return obj, nil
}

func getAllNodesWithLabel(r *ReconcileSpecialResource, label string) []corev1.Node {
	// We need the node labels to fetch the correct container
	opts := &client.ListOptions{}
	opts.SetLabelSelector(label)
	list := &corev1.NodeList{}
	err := r.client.List(context.TODO(), opts, list)
	if err != nil {
		log.Error(err, "Could not get NodeList")
		return nil
	}
	return list.Items
}

func kernelFullVersion(r *ReconcileSpecialResource) string {

	logger := log.WithValues("Request.Namespace", "default", "Request.Name", "Node")

	nodes := getAllNodesWithLabel(r, "feature.node.kubernetes.io/pci-10de.present=true")
	// Assuming all nodes are running the same kernel version,
	// One could easily add driver-kernel-versions for each node.
	for _, node := range nodes {
		labels := node.GetLabels()

		var ok bool
		kernelFullVersion, ok := labels["feature.node.kubernetes.io/kernel-version.full"]
		if ok {
			logger.Info(kernelFullVersion)
		} else {
			err := errors.NewNotFound(schema.GroupResource{Group: "Node", Resource: "Label"},
				"feature.node.kubernetes.io/kernel-version.full")
			logger.Info("Couldn't get kernelVersion", err)
			return ""
		}
		return kernelFullVersion
	}

	return ""

}

func getPromURLPass(obj *unstructured.Unstructured, r *ReconcileSpecialResource) (string, string, error) {

	promURL := ""
	promPass := ""

	grafSecret, err := kubeclient.CoreV1().Secrets("openshift-monitoring").Get("grafana-datasources", metav1.GetOptions{})
	if err != nil {
		log.Error(err, "")
		return promURL, promPass, err
	}

	promJSON := grafSecret.Data["prometheus.yaml"]

	sec := &unstructured.Unstructured{}

	if err := json.Unmarshal(promJSON, &sec.Object); err != nil {
		log.Error(err, "UnmarshlJSON")
		return promURL, promPass, err
	}

	datasources, found, err := unstructured.NestedSlice(sec.Object, "datasources")
	checkNestedFields(found, err)

	for _, datasource := range datasources {
		switch datasource := datasource.(type) {
		case map[string]interface{}:
			promURL, found, err = unstructured.NestedString(datasource, "url")
			checkNestedFields(found, err)
			promPass, found, err = unstructured.NestedString(datasource, "basicAuthPassword")
			checkNestedFields(found, err)
		default:
			log.Info("PROM", "DEFAULT NOT THE CORRECT TYPE", promURL)
		}
		break
	}

	return promURL, promPass, nil
}

func prefixNVIDIAgrafanaConfigMap(obj *unstructured.Unstructured, r *ReconcileSpecialResource) (interface{}, error) {

	promData, found, err := unstructured.NestedString(obj.Object, "data", "ocp-prometheus.yml")
	checkNestedFields(found, err)

	promURL, promPass, err := getPromURLPass(obj, r)
	if err != nil {
		return nil, err
	}

	promData = strings.Replace(promData, "REPLACE_PROM_URL", promURL, -1)
	promData = strings.Replace(promData, "REPLACE_PROM_PASS", promPass, -1)
	promData = strings.Replace(promData, "REPLACE_PROM_USER", "internal", -1)

	//log.Info("PROM", "DATA", promData)
	if err := unstructured.SetNestedField(obj.Object, promData, "data", "ocp-prometheus.yml"); err != nil {
		log.Error(err, "Couldn't update ocp-prometheus.yml")
		return nil, err
	}

	return obj, nil
}

func prefixNVIDIAdriverValdiation(obj *unstructured.Unstructured, r *ReconcileSpecialResource) (interface{}, error) {

	list := &unstructured.UnstructuredList{}
	name := obj.GetName()
	nodes := getAllNodesWithLabel(r, "feature.node.kubernetes.io/pci-10de.present=true")

	list.SetAPIVersion("v1")
	list.SetKind("PodList")
	list.
	for _, node := range nodes {
		checksum := adler32.Checksum([]byte(node.GetName()))
		strsum := strconv.FormatUint(uint64(checksum), 16)
		log.Info("PREFIX", "NodeName", node.GetName(), "ADLER32", strsum)

		fullName := name + "-" + strsum

		add := obj.DeepCopy()

		if err := unstructured.SetNestedField(add.Object, fullName, "metadata", "name"); err != nil {
			log.Error(err, "Couldn't update Pod with full name")
			return nil, err
		}

		list.Items = append(list.Items, *add)

		for i := range list.Items {
			log.Info("ITEMS", "INNER", list.Items[i].GetName())
		}

	}

	list.Object["items"] = list.Items

	for i := range list.Items {
		log.Info("ITEMS", "OUTER", list.Items[i].GetName())
	}

	return list, nil
}
