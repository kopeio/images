.PHONY: images kope

default: images

kope:
	go install github.com/kopeio/kope/registry/...

images: kope
	cd images/kope-registry; make
