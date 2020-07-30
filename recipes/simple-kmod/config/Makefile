simple-kmod: 
	-kubectl create ns $@
	-kubectl delete configmap $@ -n $@  2>/dev/null | true
	kubectl create configmap $@ -n $@ --from-file=recipes/$@/manifests
	-oc policy add-role-to-user system:image-puller system:serviceaccount:$@:builder -n driver-container-base
	@$(TEMPLATE_CMD) recipes/$@/config/$@-cr.yaml | kubectl apply -f - 