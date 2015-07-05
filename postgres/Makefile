.PHONY: images
default: images

code:
	go install github.com/kopeio/kope/postgres/...

images: code
	cd images/kope-postgres; make
