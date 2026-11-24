#!/bin/bash

script_dir="$(cd "$(dirname "$0")" && pwd)"

cd "$script_dir" || exit 1

# 设置变量
IMAGE_NAME="hkccr.ccs.tencentyun.com/kol-base/package:v1.0.1" # Docker镜像名称
CONTAINER_NAME="package"                                        # 容器名称
HOST_DIR="/home/code"                                           # 主机上的目录
CONTAINER_DIR="/home/code"                                      # 容器内的挂载点
IMAGE="hkccr.ccs.tencentyun.com"

function usage {
  echo "Usage: $0 [options]"
  echo "Options:"
  echo "  -h    Display this help message"
  echo "  -e    Choose an environment to building, like test prod"
  echo "  -v    Specify the version number for the online version"
  echo
  exit 0
}

while getopts ":m:e:v:h" opt; do
  case $opt in
  e)
    environment="$OPTARG"
    ;;
  v)
    version="$OPTARG"
    ;;
  h)
    usage
    ;;
  \?)
    echo "未知选项: -$OPTARG" >&2
    exit 1
    ;;
  :)
    echo "选项 -$OPTARG 需要一个参数" >&2
    exit 1
    ;;
  esac
done

if [[ -z "$environment" ]]; then
  echo "请指定打包环境"
  exit 1
fi

if [[ -z "$version" ]]; then
  echo "请指定版本号"
  exit 1
fi

# 检查Docker是否运行
if ! docker info >/dev/null 2>&1; then
  echo "Docker守护进程未运行，请先启动Docker"
  exit 1
fi

# 检查主机目录是否存在
if [ ! -d "$HOST_DIR" ]; then
  echo "主机目录 $HOST_DIR 不存在，正在创建..."
  mkdir -p "$HOST_DIR"
fi

docker login hkccr.ccs.tencentyun.com --username=100043371532 --password '4*$OEgYrSN#Ve$xt'

function compile {
  command="bash /home/code/joincare_frontend/joincare-web/docker/docker.sh ${environment}"

  # 运行Docker容器
  echo "正在启动Docker容器..."
  # 如果容器不存在，则创建
  if ! docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "创建新容器: $CONTAINER_NAME"
    docker run --name "$CONTAINER_NAME" \
      -v "$HOST_DIR:$CONTAINER_DIR" \
      -d "$IMAGE_NAME" \
      tail -f /dev/null
  fi

  # 启动容器（如果已停止）
  docker start "$CONTAINER_NAME" >/dev/null 2>&1

  # 在容器中执行命令
  docker exec "$CONTAINER_NAME" /bin/bash -c "$command"

  # 检查执行结果
  if [ $? -eq 0 ]; then
    echo "容器执行成功完成"
  else
    echo "容器执行失败"
    exit 1
  fi
}

function build {
  environment=$1
  version=$2

  cd "$script_dir"/../output || exit
  docker build -t "$IMAGE"/jc-"$environment"/frontend:"$version" -f "./Dockerfile" .
  docker push "$IMAGE"/jc-"$environment"/frontend:"$version"
}

compile
build  "$environment" "$version"
if [[ "$environment" == "test" ]]; then
  kubectl set image deployment/jc-frontend jc-frontend="$IMAGE"/jc-"$environment"/frontend:"$version"
fi
