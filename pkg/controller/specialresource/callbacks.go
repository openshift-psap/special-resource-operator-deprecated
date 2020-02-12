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
var postfixCallback resourceCallbacks
var waitFor resourceCallbacks

// SetupCallbacks preassign callbacks for manipulating and handling of resources
func SetupCallbacks() error {

	prefixCallback = make(resourceCallbacks)
	postfixCallback = make(resourceCallbacks)

	waitFor = make(resourceCallbacks)

	//	prefixCallback["nvidia-driver-daemonset"] = prefixNVIDIAdriverDaemonset
	prefixCallback["nvidia-grafana-configmap"] = prefixNVIDIAgrafanaConfigMap
	//	prefixCallback["nvidia-driver-internal"] = prefixNVIDIABuildConfig

	waitFor["Pod"] = waitForPod
	waitFor["DaemonSet"] = waitForDaemonSet
	waitFor["BuildConfig"] = waitForBuild
	waitFor["nvidia-driver-daemonset"] = waitForDaemonSetLogs

	return nil
}

func checkNestedFields(found bool, err error) {
	if !found || err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
}

func prefixResourceCallback(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error {

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

func postfixResourceCallback(obj *unstructured.Unstructured, r *ReconcileSpecialResource) error {

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

	/*
		todo = annotations["callback"]

		if err := waitForResource(obj, r); err != nil {
			return err
		}

		if todo, ok = annotations["callback"]; !ok {
			return nil
		}

		if postfix, ok := postfixCallback[todo]; ok {
			return postfix(obj, r)
		}
	*/
	return nil
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
	log.Info("checkForImagePullBackOff get pods1")

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
		waitings, found, err := unstructured.NestedSlice(pod.Object, "status", "containerStatuses", "state", "waiting")
		checkNestedFields(found, err)

		for _, waiting := range waitings {
			switch waiting := waiting.(type) {
			case map[string]interface{}:
				reason, found, err = unstructured.NestedString(waiting, "reason")
				log.Info("Reason", "reason", reason)
			default:
				log.Info("checkForImagePullBackOff", "DEFAULT NOT THE CORRECT TYPE", waiting.(type))
			}
			break
		}

		if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
			log.Info("ImagePullBackOff")
			annotations := obj.GetAnnotations()
			if vendor, ok := annotations["specialresource.openshift.io/driver-container-vendor"]; ok {
				updateVendor = vendor
				return fmt.Errorf("ImagePullBackOff need to rebuild %s driver-container", updateVendor)
			}
		}
	}

	return nil
}
