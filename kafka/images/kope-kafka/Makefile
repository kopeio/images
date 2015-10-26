image:
	./build.sh
	cp ${GOPATH}/bin/kope-kafka .build/kope-kafka
	cp -r ${GOPATH}/src/github.com/kopeio/kope/kafka/templates/ .build/
	docker build -t kope/kafka .
