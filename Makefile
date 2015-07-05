.PHONY: mongodb etcd memcached registry push postgres zookeeper baseimages

default: all images

all:
	go install github.com/kopeio/kope/registry/...

images: baseimages postgres memcached registry etcd mongodb zookeeper

baseimages:
	cd baseimages; make

memcached:
	cd memcached; make

etcd:
	cd etcd; make

registry:
	cd registry; make

postgres:
	cd postgres; make

mongodb:
	cd mongodb; make

zookeeper:
	cd zookeeper; make

push: images
	cd baseimages; make push
	docker push kope/registry
	docker push kope/memcached
	docker push kope/postgres
	docker push kope/etcd
	docker push kope/mongodb
	docker push kope/zookeeper
