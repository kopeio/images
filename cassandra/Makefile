.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/cassandra/...

images: code
	cd images/kope-cassandra; make

push: images
	docker push kope/cassandra
