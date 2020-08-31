package specialresource

import (
	"context"

	srov1alpha1 "github.com/openshift-psap/special-resource-operator/pkg/apis/sro/v1alpha1"
	errs "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func getSpecialResourceFrom(dependency srov1alpha1.SpecialResourceDependency, r *ReconcileSpecialResource) (srov1alpha1.SpecialResource, error) {

	// Fetch the SpecialResource instance
	specialresource := srov1alpha1.SpecialResource{}

	log.Info("Looking for SpecialResource", "Name", dependency.Name, "Namespace", dependency.Namespace)

	namespacedName := types.NamespacedName{Namespace: dependency.Namespace, Name: dependency.Name}
	err := r.client.Get(context.TODO(), namespacedName, &specialresource)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			return specialresource, errs.Wrap(err, "Is not found Error")
		}
		if apierrors.IsForbidden(err) {
			return specialresource, errs.Wrap(err, "Forbidden check Role, ClusterRole and Bindings for operator")
		}

		// Error reading the object
		return specialresource, errs.Wrap(err, "Unexpected error")
	}

	return specialresource, nil
}

func createSpecialResourceFrom(dependency srov1alpha1.SpecialResourceDependency, r *ReconcileSpecialResource) (srov1alpha1.SpecialResource, error) {

	specialresource := srov1alpha1.SpecialResource{}

	crpath := "/opt/sro/recipes/" + dependency.Name + "/config"
	manifests := getAssetsFrom(crpath)

	for _, manifest := range manifests {
		log.Info("TODO", "Creation of SpecialResource, please do it manually", manifest.name)
		/*
			log.Info("Creating", "SpecialResource", manifest.name)

			if err := createFromYAML(manifest.content, r, r.specialresource.Namespace); err != nil {
				log.Info("Cannot reconcile specialresource namespace, something went horribly wrong")
				exitOnError(err)
			}
			// Only one CR creation if they are more ignore all others
			// makes no sense to create multiple CRs for the same specialresource
			break
		*/
	}

	return specialresource, nil
}
