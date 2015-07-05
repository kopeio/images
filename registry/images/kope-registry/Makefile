default: image

registry:
	./build-docker-registry.sh

image: registry
	cp ${GOPATH}/bin/kope-registry .build/kope-registry
	cp -r ${GOPATH}/src/github.com/kopeio/kope/registry/templates/ .build/
	docker build -t kope/registry .
