#!/bin/bash
script_dir="$(cd "$(dirname "$0")" && pwd)"

cd "$script_dir" ||  exit 1

environment=$1

rm -rf /home/code/joincare_frontend/joincare-web/output
mkdir -p /home/code/joincare_frontend/joincare-web/output/

cd /home/code/joincare_frontend/joincare-web || exit 1
node scripts/switch-env.js $environment
npm install
npm run build

# 复制编译产物和运行时所需的文件到 output
# Vite 编译产物在 dist 目录（纯静态文件）
cp -r /home/code/joincare_frontend/joincare-web/dist /home/code/joincare_frontend/joincare-web/output/dist
cp /home/code/joincare_frontend/joincare-web/docker/nginx.conf /home/code/joincare_frontend/joincare-web/output/nginx.conf
cp /home/code/joincare_frontend/joincare-web/docker/entrypoint /home/code/joincare_frontend/joincare-web/output/entrypoint
cp /home/code/joincare_frontend/joincare-web/docker/Dockerfile /home/code/joincare_frontend/joincare-web/output/Dockerfile
