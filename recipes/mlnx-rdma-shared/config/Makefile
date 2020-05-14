mlnx-rdma-shared: 
	-kubectl delete configmap -n $(NAMESPACE) $@  2>/dev/null | true
	kubectl create configmap $@ -n $(NAMESPACE) --from-file=recipes/$@/manifests/rdma-shared
	@$(TEMPLATE_CMD) recipes/$@/config/$@-cr.yaml | kubectl apply -f - 

