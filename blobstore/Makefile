.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/blobstore/...

run:
	go run ${GOPATH}/src/github.com/kopeio/kope/blobstore/cmd/kope-blobstore/main.go --v=2 --logtostderr

images: code
	cd images/kope-blobstore; make

push: images
	docker push kope/blobstore
