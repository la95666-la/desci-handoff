#!/bin/bash
script_dir="$(cd "$(dirname "$0")" && pwd)"

cd "$script_dir" ||  exit 1

environment=$1

mkdir -p /home/code/joincare_server/output/notice
mkdir -p /home/code/joincare_server/output/common

cd /home/code/joincare_server || exit 1

go env -w GOPROXY=https://goproxy.cn,direct \
  && cd /home/code/joincare_server && go mod tidy \
  && cd /home/code/joincare_server/cmd/notice \
  && GOEXPERIMENT=jsonv2 go build -o /home/code/joincare_server/output/notice/notice

cp /home/code/joincare_server/config/notice/"$environment".yaml /home/code/joincare_server/output/notice/config.yaml
cp /home/code/joincare_server/config/common/"$environment".yaml /home/code/joincare_server/output/common/config.yaml
cp -r /home/code/joincare_server/config/common/i18n /home/code/joincare_server/output/common/
cp /home/code/joincare_server/build/notice/entrypoint /home/code/joincare_server/output/notice/entrypoint
cp /home/code/joincare_server/build/notice/Dockerfile /home/code/joincare_server/output/notice/Dockerfile
