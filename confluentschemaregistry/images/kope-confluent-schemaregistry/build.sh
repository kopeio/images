#!/bin/bash

mkdir -p .build
pushd .build
wget -N http://packages.confluent.io/archive/1.0/confluent-1.0.1-2.10.4.tar.gz
popd
