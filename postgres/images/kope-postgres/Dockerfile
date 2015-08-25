FROM debian:jessie

# Install Postgres from the official PG repositories
# See http://www.postgresql.org/download/linux/debian/
COPY postgres.list /etc/apt/source/list.d/postgres.list
COPY ACCC4CF8.asc /tmp/postgres.asc
RUN 	apt-key add /tmp/postgres.asc && \
	rm /tmp/postgres.asc

# Postgres picks up current system locale; set to en_US.UTF-8
RUN	apt-get update && \
	apt-get install --no-install-recommends -y locales
COPY	locale.gen /etc/locale.gen
RUN	/usr/sbin/locale-gen

# Install postgresql-common which sets up /etc/postgresql-common/createcluster.conf
RUN 	apt-get update && \
	apt-get install --no-install-recommends -y postgresql-common

# Replace createcluster with a configuration that won't create a cluster (DB)
# We want to wait till first launch.
COPY	nocreatecluster.conf /etc/postgresql-common/createcluster.conf

# Install PG server itself
RUN	apt-get install --no-install-recommends -y postgresql-9.4 postgresql-contrib-9.4

COPY .build/templates/ /templates/
COPY .build/kope-postgres /
CMD /kope-postgres --logtostderr -v=2
