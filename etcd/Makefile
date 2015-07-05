.PHONY: images kope

default: images

kope:
	go install github.com/kopeio/kope/etcd/...

images: kope
	cd images/kope-etcd; make
