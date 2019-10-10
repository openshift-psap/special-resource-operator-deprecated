package specialresource

import (
	"os"
	"reflect"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type callback func(obj *unstructured.Unstructured, r *ReconcileSpecialResource) (interface{}, error)
type resourceCallbacks map[string]callback

var prefixCallback resourceCallbacks
var postfixCallback resourceCallbacks

// SetupCallbacks preassign callbacks for manipulating and handling of resources
func SetupCallbacks() error {

	prefixCallback = make(resourceCallbacks)
	postfixCallback = make(resourceCallbacks)

	prefixCallback["nvidia-driver-daemonset"] = prefixNVIDIAdriverDaemonset
	prefixCallback["nvidia-driver-validation"] = prefixNVIDIAdriverValdiation
	prefixCallback["nvidia-grafana-configmap"] = prefixNVIDIAgrafanaConfigMap
	prefixCallback["nvidia-device-plugin-validation"] = prefixNVIDIAdevicePluginValidation

	return nil
}

func checkNestedFields(found bool, err error) {
	if !found || err != nil {
		log.Error(err, "")
		os.Exit(1)
	}
}

func getCallback(obj *unstructured.Unstructured, fn resourceCallbacks) (callback, bool) {

	var ok bool
	todo := ""
	annotations := obj.GetAnnotations()

	if todo, ok = annotations["callback"]; !ok {
		return nil, false
	}

	if prefix, ok := fn[todo]; ok {
		return prefix, ok
	}
	return nil, false
}

func prefixResourceCallback(obj interface{}, r *ReconcileSpecialResource) (interface{}, error) {

	switch res := obj.(type) {
	case *unstructured.Unstructured:
		if prefix, ok := getCallback(res, prefixCallback); ok {
			return prefix(res, r)
		}
	case *unstructured.UnstructuredList:
		log.Info("DEFAULT", "Lists not yet supported prefix", res.GetKind())
	default:
		log.Info("DEFAULT", "No idea what to do with: ", reflect.TypeOf(obj))
	}
	return obj, nil

}

func postfixResourceCallback(obj interface{}, r *ReconcileSpecialResource) (interface{}, error) {

	switch res := obj.(type) {
	case *unstructured.Unstructured:
		if postfix, ok := getCallback(res, postfixCallback); ok {
			return postfix(res, r)
		}
		// If there is no postfix callback per default we wait for any resource
		if err := waitForResource(res, r); err != nil {
			return obj, err
		}

	case *unstructured.UnstructuredList:
		for _, item := range res.Items {

			if err := waitForResource(&item, r); err != nil {
				return obj, err
			}
		}

	default:
		log.Info("DEFAULT", "No idea what to do with: ", reflect.TypeOf(obj))
	}

	return obj, nil
}
