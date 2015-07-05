FROM debian:jessie

RUN groupadd -r mongodb && useradd -r -g mongodb mongodb

RUN apt-get update && apt-get install --yes --no-install-recommends libssl1.0.0
ADD .build/mongodb-linux-x86_64-debian71-3.0.4.tgz /opt
RUN mv /opt/mongodb-linux-x86_64-debian71-3.0.4/ /opt/mongodb/ && chown -R root:root /opt/mongodb

COPY .build/templates/ /templates/
COPY .build/kope-mongodb /
CMD /kope-mongodb --logtostderr -v=2