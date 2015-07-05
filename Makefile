.PHONY: etcd memcached registry push postgres

default: all images

all:
	go install github.com/kopeio/kope/registry/...

images: postgres memcached registry etcd

memcached:
	cd memcached; make

etcd:
	cd etcd; make

registry:
	cd registry; make

postgres:
	cd postgres; make

push: images
	docker push kope/registry
	docker push kope/memcached
	docker push kope/postgres
	docker push kope/etcd
