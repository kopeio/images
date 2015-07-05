default: image

registry:
	./build-etcd.sh

image: registry
	cp ${GOPATH}/bin/kope-etcd .build/kope-etcd
	cp -r ${GOPATH}/src/github.com/kopeio/kope/etcd/templates/ .build/
	docker build -t kope/etcd .
