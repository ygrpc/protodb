#!/bin/bash

protoc --go_out=. protodb.proto
mv -f github.com/ygrpc/protodb/protodb.pb.go .
rm -rf github.com

if [ -d "../ygrpctest/cmd/protodb" ]; then
  echo "sync protodb.proto to ../ygrpctest/cmd/protodb/"
  rsync -ac --stats protodb.proto ../ygrpctest/cmd/protodb/ | grep files
fi
