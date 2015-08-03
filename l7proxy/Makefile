.PHONY: images push
default: images

code:
	go install github.com/kopeio/kope/l7proxy/...

run:
	go run ${GOPATH}/src/github.com/kopeio/kope/l7proxy/cmd/kope-l7proxy/main.go --v=2 --logtostderr

images: code
	cd images/kope-l7proxy; make

push: images
	docker push kope/l7proxy
