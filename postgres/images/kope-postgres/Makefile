all: images

keys:
	wget -nc https://www.postgresql.org/media/keys/ACCC4CF8.asc

images:
	cp ${GOPATH}/bin/kope-postgres .build/kope-postgres
	cp -r ${GOPATH}/src/github.com/kopeio/kope/postgres/templates/ .build/
	docker build -t kope/postgres .
