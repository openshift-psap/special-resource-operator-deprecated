nvidia-gpu:  
	-kubectl delete configmap -n $(NAMESPACE) $@ 2>/dev/null | true
	kubectl create configmap $@ -n $(NAMESPACE) --from-file=recipes/$@/manifests
	@$(TEMPLATE_CMD) recipes/$@/config/$@_specialresource_cr.yaml | kubectl apply -f - 

