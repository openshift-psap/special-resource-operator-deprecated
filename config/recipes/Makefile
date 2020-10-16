$(SPECIALRESOURCE): 
	kubectl apply -f config/recipes/$(SPECIALRESOURCE)/0000-$(SPECIALRESOURCE)-cr.yaml

$(SPECIALRESOURCE)_manifests:
	cd config/recipes/$(SPECIALRESOURCE)/manifests && kustomize edit set namespace $@
	kubectl apply -k config/recipes/$(SPECIALRESOURCE)/manifests
