
nvidia-gpu: deploy 
	-kubectl delete configmap -n $(NAMESPACE) -l specialresource.openshift.io/config=true
	kubectl create configmap 0000-$@ -n $(NAMESPACE) --from-file=recipes/$@/manifests
	kubectl patch configmap 0000-$@ -n $(NAMESPACE)  --patch "$(cat recipes/$@/config/patch.yaml)" 

