FROM kope/java7

ADD .build/zookeeper-3.4.6.tar.gz /opt/
RUN mv /opt/zookeeper-3.4.6 /opt/zk

COPY .build/templates/ /templates/
COPY .build/kope-zookeeper /
CMD /kope-zookeeper --logtostderr -v=2