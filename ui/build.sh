#!/bin/sh
# This script is intended to be run inside the builder container

set -e
SRCROOT=`dirname $0`
SRCROOT=`cd $SRCROOT;pwd`

# Tried to be fancy by manipulating NODE_PATH before running the build, but some utilities (e.g webpack)
# as well as npm installed asset files aren't able to resolve libraries and asset locations cleanly.
# For now, we blow away any node_modules directory in the ui dir, and symlink the node_modules that are
# already baked into this container.
#export NODE_PATH="/usr/lib/node_modules/claudia/node_modules"
#export PATH="/usr/lib/node_modules/claudia/node_modules/.bin:$PATH"
cd $SRCROOT
rm -rf $SRCROOT/node_modules
ln -s /root/node/node_modules
npm run build:prod
