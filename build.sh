#!/bin/sh
# Build the binary in native platform. This script is invoked inside the builder container
# as well as for development purposes (e.g. Mac)

set -xe
SRCROOT=$(cd $(dirname $0);pwd)

PACKAGE_NAME="github.com/applatix/claudia"

echo "Linting source"
gometalinter --config ${SRCROOT}/gometalinter.json --deadline 2m ${SRCROOT}/...

CLAUDIA_VERSION=$(cat $SRCROOT/VERSION)
CLAUDIA_REVISION=$(git -C $SRCROOT rev-parse --short=7 HEAD)
dirty=$(git -C $SRCROOT status --porcelain)
if [ ! -z "$dirty" ]; then
    CLAUDIA_REVISION="${CLAUDIA_REVISION}-dirty"
fi
BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%S')

# GOBIN is manupulated so that binaries end up in the project directory,
# so that we can subsequently package it into a container.
export GOBIN="$SRCROOT/dist/bin"
# -pkgdir is used to allow repeat builds to complete 
PKGDIR="$SRCROOT/dist/$(go env GOOS)_$(go env GOARCH)"

LD_FLAGS="-X $PACKAGE_NAME.Version=$CLAUDIA_VERSION -X $PACKAGE_NAME.Revision=$CLAUDIA_REVISION -X $PACKAGE_NAME.BuildDate=$BUILD_DATE"
# purge any precompiled claudia archive files in order to guarantee version
# and build date information is updated in the resulting binaries.
# `go install` will decide to skip the build if it thinks there were no
# source code changes, despite passing different LD_FLAGS.
rm -rf "$PKGDIR/$PACKAGE_NAME"
go env
go install -v -pkgdir "$PKGDIR" -ldflags "$LD_FLAGS" $PACKAGE_NAME/claudiad
go install -v -pkgdir "$PKGDIR" -ldflags "$LD_FLAGS" $PACKAGE_NAME/ingestd
