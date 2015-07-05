image:
	cp ${GOPATH}/bin/kope-postgres .build/kope-postgres
	cp -r ${GOPATH}/src/github.com/kopeio/kope/postgres/templates/ .build/
	docker build -t kope/postgres .
