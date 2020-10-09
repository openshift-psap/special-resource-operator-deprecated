package controllers

import (
	"context"
	"fmt"

	srov1beta1 "github.com/openshift-psap/special-resource-operator/api/v1beta1"
	errs "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// +kubebuilder:rbac:groups=sro.openshift.io,resources=specialresources,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sro.openshift.io,resources=specialresources/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sro.openshift.io,resources=specialresources/finalizers,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get
// +kubebuilder:rbac:groups=config.openshift.io,resources=proxies,verbs=get;list
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list

// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=image.openshift.io,resources=imagestreams/finalizers,verbs=get;list;watch;create;update;patch;delete

func SpecialResourceReconcilers(r *SpecialResourceReconciler, req ctrl.Request) (ctrl.Result, error) {

	log.Info("Reconciling SpecialResource(s) in all Namespaces")
	specialresources := &srov1beta1.SpecialResourceList{}

	opts := []client.ListOption{
		client.InNamespace(req.NamespacedName.Namespace),
	}
	err := r.List(context.TODO(), specialresources, opts...)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	for _, specialresource := range specialresources.Items {

		//namespacedName := types.NamespacedName{Namespace: specialresource.Spec.Metadata.Namespace, Name: specialresource.Name}
		//log = r.Log.WithValues("specialresource", namespacedName)
		log.Info("Reconciling Dependencies of")

		// Only one level dependency support for now
		for _, dependency := range specialresource.Spec.DependsOn {

			//namespacedName := types.NamespacedName{Namespace: dependency.Namespace, Name: dependency.Name}
			//log = r.Log.WithValues("specialresource", namespacedName)
			log.Info("Fetching Dependency")

			if r.specialresource, err = getSpecialResourceFrom(dependency, r); err != nil {
				log.Info("Could not get SpecialResource dependency", "error", fmt.Sprintf("%v", err))
				if r.specialresource, err = createSpecialResourceFrom(dependency, r); err != nil {
					return reconcile.Result{}, errs.New("Dependency creation failed")
				}
			}

			log.Info("Reconciling Dependency")
			if err := ReconcileHardwareConfigurations(r); err != nil {
				// We do not want a stacktrace here, errs.Wrap already created
				// breadcrumb of errors to follow. Just sprintf with %v rather than %+v
				log.Info("Could not reconcile hardware configurations", "error", fmt.Sprintf("%v", err))
				return reconcile.Result{}, errs.New("Reconciling failed")
			}
		}

		log.Info("Reconciling")

		r.specialresource = specialresource
		if err := ReconcileHardwareConfigurations(r); err != nil {
			// We do not want a stacktrace here, errs.Wrap already created
			// breadcrumb of errors to follow. Just sprintf with %v rather than %+v
			log.Info("Could not reconcile hardware configurations", "error", fmt.Sprintf("%v", err))
			return reconcile.Result{}, errs.New("Reconciling failed")
		}

	}

	return reconcile.Result{}, nil

}

func getSpecialResourceFrom(dependency srov1beta1.SpecialResourceDependency, r *SpecialResourceReconciler) (srov1beta1.SpecialResource, error) {

	// Fetch the SpecialResource instance
	specialresource := srov1beta1.SpecialResource{}

	log.Info("Looking for SpecialResource", "Name", dependency.Name, "Namespace", dependency.Namespace)

	namespacedName := types.NamespacedName{Namespace: dependency.Namespace, Name: dependency.Name}
	err := r.Get(context.TODO(), namespacedName, &specialresource)

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

func createSpecialResourceFrom(dependency srov1beta1.SpecialResourceDependency, r *SpecialResourceReconciler) (srov1beta1.SpecialResource, error) {

	specialresource := srov1beta1.SpecialResource{}

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
