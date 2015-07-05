FROM debian:jessie

RUN apt-get update; apt-get install --yes --no-install-recommends postgresql-9.4

COPY .build/templates/ /templates/
COPY .build/kope-postgres /
CMD /kope-postgres --logtostderr -v=2