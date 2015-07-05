.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/postgres/...

images: code
	cd images/kope-postgres; make

push: images
	docker push kope/postgres
