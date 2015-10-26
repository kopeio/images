.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/kafka/...

images: code
	cd images/kope-kafka; make

push: images
	docker push kope/kafka
