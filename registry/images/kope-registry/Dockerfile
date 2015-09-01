FROM debian:jessie

RUN groupadd -r registry && useradd -r -g registry registry

COPY .build/templates/ /templates/
COPY .build/kope-registry /
COPY .build/opt/registry /opt/registry

# TODO: Figure out how to mount /data as a user
#RUN mkdir /data && chown registry:registry /data
#USER registry

CMD /kope-registry --logtostderr -v=2
