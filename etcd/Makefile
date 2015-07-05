.PHONY: images kope push

default: images

kope:
	go install github.com/kopeio/kope/etcd/...

images: kope
	cd images/kope-etcd; make

push: images
	docker push kope/etcd
