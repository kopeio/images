.PHONY: mongodb image

default: image

mongodb:
	./download.sh

image: mongodb
	cp ${GOPATH}/bin/kope-mongodb .build/kope-mongodb
	cp -r ${GOPATH}/src/github.com/kopeio/kope/mongodb/templates/ .build/
	docker build -t kope/mongodb .
