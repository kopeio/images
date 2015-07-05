.PHONY: images kope push

default: images

kope:
	go install github.com/kopeio/kope/registry/...

images: kope
	cd images/kope-registry; make

push: images
	docker push kope/registry
