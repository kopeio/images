.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/confluentschemaregistry/...

images: code
	cd images/kope-confluent-schemaregistry; make

push: images
	docker push kope/confluent-schemaregistry
