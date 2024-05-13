#!/bin/bash

protoc --go_out=. protodb.proto
mv -f github.com/ygrpc/protodb/protodb.pb.go .
rm -rf github.com
