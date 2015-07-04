.PHONY: memcached

memcached:
	go install github.com/kopeio/kope/memcached/...
	cd memcached; make
