#!/bin/bash

protoc --go_out=paths=source_relative:. \
  --connect-go_out=. --connect-go_opt=paths=source_relative,package_suffix="" \
  protodb.proto


if [ -d "../ygrpctest/cmd/protodb" ]; then
  echo "sync protodb.proto to ../ygrpctest/cmd/protodb/"
  rsync -ac --stats protodb.proto ../ygrpctest/cmd/protodb/ | grep files
fi
