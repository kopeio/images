image:
	./build.sh
	cp ${GOPATH}/bin/kope-cassandra .build/kope-cassandra
	cp -r ${GOPATH}/src/github.com/kopeio/kope/cassandra/templates/ .build/
	docker build -t kope/cassandra .
