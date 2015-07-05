.PHONY: memcached registry push

images: memcached registry

memcached:
	cd memcached; make

registry:
	cd registry; make

push: images
	docker push kope/registry
	docker push kope/memcached
