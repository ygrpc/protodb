#!/bin/bash

if ! command -v protoc &> /dev/null
then
  echo "Please install protoc first"
  exit 1
fi

if ! command -v protoc-gen-ygrpc-protodb &> /dev/null
then
  go install github.com/ygrpc/protocgen/cmd/protoc-gen-ygrpc-protodb@v0.1.12
fi

if ! command -v protoc-gen-connect-go &> /dev/null
then
  go install connectrpc.com/connect/cmd/protoc-gen-connect-go@v1.18
fi

if ! command -v protoc-gen-go &> /dev/null
then
  go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36
fi

# build proto files in proto dir

protoc  -Iproto --go_out=.. --ygrpc-protodb_out=.. ./proto/db.proto
protoc  -Iproto --go_out=.. --connect-go_out=.. --connect-go_opt=package_suffix="" ./proto/bfmap.rpc.proto

cd webclient
./build_proto.sh
cd -