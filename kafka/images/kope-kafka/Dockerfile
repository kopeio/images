FROM kope/java7

ADD .build/kafka_2.10-0.8.2.2.tgz /opt/
RUN mv /opt/kafka_2.10-0.8.2.2 /opt/kafka

COPY .build/templates/ /templates/
COPY .build/kope-kafka /
CMD /kope-kafka --logtostderr -v=2