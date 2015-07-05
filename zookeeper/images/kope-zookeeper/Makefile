image:
	./build.sh
	cp ${GOPATH}/bin/kope-zookeeper .build/kope-zookeeper
	cp -r ${GOPATH}/src/github.com/kopeio/kope/zookeeper/templates/ .build/
	docker build -t kope/zookeeper .
