.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/zookeeper/...

images: code
	cd images/kope-zookeeper; make

push: images
	docker push kope/zookeeper
