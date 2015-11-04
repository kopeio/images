FROM kope/java7

ADD .build/confluent-1.0.1-2.10.4.tar.gz /opt/
RUN mv /opt/confluent-1.0.1 /opt/confluent

COPY .build/templates/ /templates/
COPY .build/kope-confluentschemaregistry /
CMD /kope-confluentschemaregistry --logtostderr -v=2