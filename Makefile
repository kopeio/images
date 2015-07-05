.PHONY: mongodb etcd memcached registry push postgres zookeeper baseimages

default: all images

all:
	go install github.com/kopeio/kope/...

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
	cd registry; make push
	cd memcached; make push
	cd postgres; make push
	cd etcd; make push
	cd mongodb; make push
	cd zookeeper; make push
