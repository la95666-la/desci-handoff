#!/bin/bash
script_dir="$(cd "$(dirname "$0")" && pwd)"

cd "$script_dir" ||  exit 1

environment=$1

mkdir -p /home/code/joincare_server/output/job
mkdir -p /home/code/joincare_server/output/common
mkdir -p /home/code/joincare_server/output/secret

cd /home/code/joincare_server || exit 1

go env -w GOPROXY=https://goproxy.cn,direct \
  && cd /home/code/joincare_server && go mod tidy \
  && cd /home/code/joincare_server/cmd/job \
  && GOEXPERIMENT=jsonv2 go build -ldflags "-s -w -X joincare_server/pkg/job/flag.token=9CYcWtoME3YbTOWLTFBCB7r7WcmHkJMi" \
    -o /home/code/joincare_server/output/job/job

cp /home/code/joincare_server/config/job/"$environment".yaml /home/code/joincare_server/output/job/config.yaml
cp /home/code/joincare_server/config/common/"$environment".yaml /home/code/joincare_server/output/common/config.yaml
cp -r /home/code/joincare_server/config/common/i18n /home/code/joincare_server/output/common/
cp /home/code/joincare_server/config/secret/"$environment".yaml /home/code/joincare_server/output/secret/config.yaml
cp /home/code/joincare_server/build/job/entrypoint /home/code/joincare_server/output/job/entrypoint
cp /home/code/joincare_server/build/job/Dockerfile /home/code/joincare_server/output/job/Dockerfile
