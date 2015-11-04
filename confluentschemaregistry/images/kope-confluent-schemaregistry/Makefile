image:
	./build.sh
	cp ${GOPATH}/bin/kope-confluentschemaregistry .build/kope-confluentschemaregistry
	cp -r ${GOPATH}/src/github.com/kopeio/kope/confluentschemaregistry/templates/ .build/
	docker build -t kope/confluent-schemaregistry .
