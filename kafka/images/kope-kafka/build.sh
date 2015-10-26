#!/bin/bash

mkdir -p .build
pushd .build
wget -N https://www.apache.org/dist/kafka/0.8.2.2/kafka_2.10-0.8.2.2.tgz
popd
