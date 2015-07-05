FROM debian:jessie

COPY .build/templates/ /templates/
COPY .build/kope-etcd /
COPY .build/opt/etcd /opt/etcd
CMD /kope-etcd --logtostderr -v=2