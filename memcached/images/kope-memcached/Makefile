image:
	cp ${GOPATH}/bin/kope-memcached .build/kope-memcached
	cp -r ${GOPATH}/src/github.com/kopeio/kope/memcached/templates/ .build/
	docker build -t kope/memcached .
