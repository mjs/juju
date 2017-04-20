
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

operator_pod != kubectl get po -o name | cut -d '/' -f 2 | grep juju-operator

update-operator: push
	kubectl exec ${operator_pod} -- kill 1
