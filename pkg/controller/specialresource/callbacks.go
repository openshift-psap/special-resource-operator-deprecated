package specialresource

import (
	"context"
	"fmt"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type resourceCallbacks map[string]func(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error

var prefixCallback resourceCallbacks
var waitFor resourceCallbacks

// SetupCallbacks preassign callbacks for manipulating and handling of resources
func SetupCallbacks() error {

	prefixCallback = make(resourceCallbacks)

	waitFor = make(resourceCallbacks)
	prefixCallback["nvidia-grafana-configmap"] = prefixNVIDIAgrafanaConfigMap

	waitFor["Pod"] = waitForPod
	waitFor["DaemonSet"] = waitForDaemonSet
	waitFor["BuildConfig"] = waitForBuild

	return nil
}

func checkNestedFields(found bool, err error) {
	if !found || err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
}

func beforeCRUDhooks(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error {

	var ok bool
	todo := ""
	annotations := obj.GetAnnotations()

	if todo, ok = annotations["callback"]; !ok {
		return nil
	}

	if prefix, ok := prefixCallback[todo]; ok {
		return prefix(obj, r)
	}
	return nil
}

func afterCRUDhooks(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error {

	annotations := obj.GetAnnotations()

	if state, ok := annotations["specialresource.openshift.io/state"]; ok && state == "driver-container" {
		if err := checkForImagePullBackOff(obj, r); err != nil {
			return err
		}
	}

	if wait, ok := annotations["specialresource.openshift.io/wait"]; ok && wait == "true" {
		if err := waitForResource(obj, r); err != nil {
			return err
		}
	}

	if pattern, ok := annotations["specialresrouce.openshift.io/wait-for-logs"]; ok && len(pattern) > 0 {
		if err := waitForDaemonSetLogs(obj, r, pattern); err != nil {
			return err
		}
	}

	// If resource available, label the nodes according to the current state
	// if e.g driver-container ready -> specialresource.openshift.io/driver-container:ready
	return labelNodesAccordingToState(obj, r)
}

func checkForImagePullBackOff(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error {

	if err := waitForDaemonSet(obj, r); err == nil {
		return nil
	}

	log.Info("checkForImagePullBackOff get pods")

	labels := obj.GetLabels()
	value := labels["app"]

	find := make(map[string]string)
	find["app"] = value

	// DaemonSet is not coming up, lets check if we have to rebuild
	pods := &unstructured.UnstructuredList{}
	pods.SetAPIVersion("v1")
	pods.SetKind("PodList")

	opts := &client.ListOptions{}
	opts.InNamespace(r.specialresource.Namespace)
	opts.MatchingLabels(find)
	log.Info("checkForImagePullBackOff get PodList")

	err := r.client.List(context.TODO(), opts, pods)
	if err != nil {
		log.Error(err, "Could not get PodList")
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("No Pods found, reconciling")
	}

	var reason string

	for _, pod := range pods.Items {
		log.Info("checkForImagePullBackOff", "PodName", pod.GetName())

		containerStatuses, found, err := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
		checkNestedFields(found, err)

		for _, containerStatus := range containerStatuses {
			switch containerStatus := containerStatus.(type) {
			case map[string]interface{}:
				reason, found, err = unstructured.NestedString(containerStatus, "state", "waiting", "reason")
				log.Info("Reason", "reason", reason)
			default:
				log.Info("checkForImagePullBackOff", "DEFAULT NOT THE CORRECT TYPE", containerStatus)
			}
			break
		}

		if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
			annotations := obj.GetAnnotations()
			if vendor, ok := annotations["specialresource.openshift.io/driver-container-vendor"]; ok {
				updateVendor = vendor
				return fmt.Errorf("ImagePullBackOff need to rebuild %s driver-container", updateVendor)
			}
		}

		log.Info("Unsetting updateVendor, Pods not in ImagePullBackOff or ErrImagePull")
		updateVendor = ""
		return nil
	}

	return nil
}
