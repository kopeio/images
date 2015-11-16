FROM kope/java7

ADD .build/apache-cassandra-2.2.3-bin.tar.gz /opt/
RUN mv /opt/apache-cassandra-2.2.3 /opt/cassandra

COPY .build/templates/ /templates/
COPY .build/kope-cassandra /
CMD /kope-cassandra --logtostderr -v=2