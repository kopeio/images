#!/bin/bash

set -ex

mkdir -p .build/go

cd .build
BASEDIR=`pwd`

if [[ -d ${BASEDIR}/go/src/github.com/coreos/etcd ]]; then
  pushd ${BASEDIR}/go/src/github.com/coreos/etcd
  git pull
  popd
else
  mkdir -p ${BASEDIR}/go/src/github.com/coreos
  pushd ${BASEDIR}/go/src/github.com/coreos
  git clone https://github.com/coreos/etcd.git
  popd
fi

cd ${BASEDIR}/go/src/github.com/coreos/etcd
./build

cd ${BASEDIR}
mkdir -p opt/etcd
cp go/src/github.com/coreos/etcd/bin/etcd opt/etcd/

echo "Done"