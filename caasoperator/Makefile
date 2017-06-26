
DOCKER_USERNAME?=mikemccracken
DOCKER_REGISTRY_SERVER=https://index.docker.io/v1/
DOCKER_EMAIL=""
DOCKER_PASSWORD=""

install-juju:
	make -C${GOPATH}/src/github.com/juju/juju install

image: install-juju
	cp ${GOPATH}/bin/jujud .
	docker build . -t ${DOCKER_USERNAME}/caasoperator


push:	image
	# docker push localhost:5000/caasoperator
	docker push ${DOCKER_USERNAME}/caasoperator


start-registry:
	echo "TODO"

start-caasoperator: push
	kubectl create -f caasoperator.yaml

rolling-update: push
	kubectl rolling-update juju-caasoperator-v0 --image=index.docker.io/mikemccracken/caasoperator:latest --image-pull-policy=Always

update-bouncepod: push
	kubectl delete `kubectl get po -o name | grep juju-caas`

add-dockerhub-secret:
	kubectl create secret docker-registry dockerhubkey \
		--docker-server=${DOCKER_REGISTRY_SERVER} \
		--docker-username=${DOCKER_USERNAME} \
		--docker-password=${DOCKER_PASSWORD} \
		--docker-email=${DOCKER_EMAIL}


local-update-restart: local-update restart-operators

test_env_var:
	@:$(if $(value JUJU_K8S_MODEL),, $(error Undefined JUJU_K8S_MODEL))
	@juju show-model ${JUJU_K8S_MODEL} > /dev/null

local-update: test_env_var image
	$(eval kubeworkers != juju status -m ${JUJU_K8S_MODEL} kubernetes-worker --format json | jq -c '.machines | keys' | tr  -c '[:digit:]' ' ' 2>&1)
	docker save ${DOCKER_USERNAME}/caasoperator | gzip > /tmp/caasoperator-image.tar.gz
	$(foreach wm,$(kubeworkers), juju scp -m ${JUJU_K8S_MODEL} /tmp/caasoperator-image.tar.gz $(wm):/tmp/caasoperator-image.tar.gz ; )
	$(foreach wm,$(kubeworkers), juju ssh -m ${JUJU_K8S_MODEL} $(wm) -- "zcat /tmp/caasoperator-image.tar.gz | docker load" ; )

operator_pods != kubectl get po -o name | cut -d '/' -f 2 | grep juju-operator
restart-operators:
	@echo restarting ${operator_pods}
	$(foreach p,$(operator_pods), kubectl exec $(p) -- kill 1 ; )
