.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/memcached/...

images: code
	cd images/kope-memcached; make

push: images
	docker push kope/memcached
