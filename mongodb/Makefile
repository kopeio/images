.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/mongodb/...

images: code
	cd images/kope-mongodb; make

push: images
	docker push kope/mongodb
