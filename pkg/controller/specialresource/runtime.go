package specialresource

import (
	"strings"

	errs "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func injectRuntimeInformation(jsonSpec *[]byte) error {

	obj := &unstructured.Unstructured{}

	err := obj.UnmarshalJSON(*jsonSpec)
	exitOnError(errs.Wrap(err, "Cannot unmarshall json spec, check your manifests"))

	annotations := obj.GetAnnotations()
	if inject, ok := annotations["specialresource.openshift.io/inject-runtime-info"]; !ok || inject != "true" {
		return nil
	}

	spec := string(*jsonSpec)

	log.Info("Runtime Information", "operatingSystem", operatingSystem)
	log.Info("Runtime Information", "kernelVersion", kernelVersion)
	log.Info("Runtime Information", "clusterVersion", clusterVersion)
	log.Info("Runtime Information", "updateVendor", updateVendor)
	log.Info("Runtime Information", "nodeFeature", nodeFeature)

	pattern := strings.NewReplacer(
		"SPECIALRESOURCE.OPENSHIFT.IO.OPERATINGSYSTEM", operatingSystem,
		"SPECIALRESOURCE.OPENSHIFT.IO.KERNELVERSION", kernelVersion,
		"SPECIALRESOURCE.OPENSHIFT.IO.CLUSTERVERSION", clusterVersion,
		"SPECIALRESOURCE.OPENSHIFT.IO.NODEFEATURE", nodeFeature)

	*jsonSpec = []byte(pattern.Replace(spec))
	return nil
}

func getOperatingSystem() (string, error) {

	var nodeOSrel string
	var nodeOSver string

	// Assuming all nodes are running the same os
	for _, node := range node.list.Items {
		labels := node.GetLabels()
		nodeOSrel = labels["feature.node.kubernetes.io/system-os_release.ID"]
		nodeOSver = labels["feature.node.kubernetes.io/system-os_release.VERSION_ID.major"]

		if len(nodeOSrel) == 0 || len(nodeOSver) == 0 {
			return "", errs.New("Cannot extract feature.node.kubernetes.io/system-os_release.*, is NFD running? Check node labels")
		}
		break
	}

	return renderOperatingSystem(nodeOSrel, nodeOSver), nil
}

func renderOperatingSystem(rel string, ver string) string {

	log.Info("OS", "rel", rel)
	log.Info("OS", "ver", ver)

	var nodeOS string

	if strings.Compare(rel, "rhcos") == 0 && strings.Compare(ver, "4") == 0 {
		log.Info("Setting OS to rhel8")
		nodeOS = "rhel8"
	}

	if strings.Compare(rel, "rhel") == 0 && strings.Compare(ver, "8") == 0 {
		log.Info("Setting OS to rhel8")
		nodeOS = "rhel8"
	}

	if strings.Compare(rel, "rhel") == 0 && strings.Compare(ver, "7") == 0 {
		log.Info("Setting OS to rhel7")
		nodeOS = "rhel7"
	}

	return nodeOS
}

func getKernelVersion() (string, error) {

	var ok bool
	var kernelVersion string
	// Assuming all nodes are running the same kernel version,
	// one could easily add driver-kernel-versions for each node.
	for _, node := range node.list.Items {
		labels := node.GetLabels()

		// We only need to check for the key, the value is available if the key is there
		if kernelVersion, ok = labels["feature.node.kubernetes.io/kernel-version.full"]; !ok {
			return "", errs.New("Label feature.node.kubernetes.io/kernel-version.full not found is NFD running? Check node labels")
		}
		break
	}

	return kernelVersion, nil
}

func getClusterVersion() (string, error) {
	return "", nil
}
