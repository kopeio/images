FROM debian:jessie

RUN apt-get update; apt-get install --yes --no-install-recommends memcached

COPY .build/templates/ /templates/
COPY .build/kope-blobstore /
CMD /kope-blobstore --logtostderr -v=2
