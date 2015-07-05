FROM debian:jessie

COPY .build/templates/ /templates/
COPY .build/kope-registry /
COPY .build/opt/registry /opt/registry
CMD /kope-registry --logtostderr -v=2