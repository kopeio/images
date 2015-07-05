.PHONY: memcached registry push postgres

default: all images

all:
	go install github.com/kopeio/kope/registry/...

images: postgres memcached registry

memcached:
	cd memcached; make

registry:
	cd registry; make

postgres:
	cd postgres; make

push: images
	docker push kope/registry
	docker push kope/memcached
	docker push kope/postgres
