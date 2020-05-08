$(SPEICALRESOURCE): 
	-kubectl delete configmap -n $(NAMESPACE) $@-mofed  2>/dev/null | true
	kubectl create configmap $@-mofed -n $(NAMESPACE) --from-file=recipes/$@/manifests/mofed
	@$(TEMPLATE_CMD) recipes/$@/config/$@_mofed_specialresource_cr.yaml | kubectl apply -f - 

	-kubectl delete configmap -n $(NAMESPACE) $@-gpudirect  2>/dev/null | true
	kubectl create configmap $@-gpudirect -n $(NAMESPACE) --from-file=recipes/$@/manifests/gpudirect
	@$(TEMPLATE_CMD) recipes/$@/config/$@_gpudirect_specialresource_cr.yaml | kubectl apply -f - 

