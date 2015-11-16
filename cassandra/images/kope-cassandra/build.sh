#!/bin/bash

mkdir -p .build
pushd .build
wget -N http://apache.osuosl.org/cassandra/2.2.3/apache-cassandra-2.2.3-bin.tar.gz
popd
