#!/bin/bash

set -ex

mkdir -p .build/go

cd .build
BASEDIR=`pwd`

if [[ -d ${BASEDIR}/go/src/github.com/docker/distribution ]]; then
  pushd ${BASEDIR}/go/src/github.com/docker/distribution
  git pull
  popd
else
  mkdir -p ${BASEDIR}/go/src/github.com/docker
  pushd ${BASEDIR}/go/src/github.com/docker
  git clone https://github.com/docker/distribution.git
  popd
fi

cd ${BASEDIR}/go/src/github.com/docker/distribution
export GOPATH=${BASEDIR}/go/src/github.com/docker/distribution/Godeps/_workspace:${BASEDIR}/go
make clean binaries

cd ${BASEDIR}
mkdir -p opt/registry
cp go/src/github.com/docker/distribution/bin/registry opt/registry/

echo "Done"