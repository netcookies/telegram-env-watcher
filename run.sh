#!/bin/bash
set -e

APP_NAME=telegram-env-watcher
SESSION_FILE="${PWD}/session.json"
CONFIG_FILE="${PWD}/config.json"
IMAGE_NAME=telegram-env-watcher:latest

# 确保 config.json 存在
if [ ! -f "$CONFIG_FILE" ]; then
  echo "❌ 缺少 config.json 文件，请先创建。"
  exit 1
fi

# 第一次登录：如果 session.json 不存在或为空，启动交互式容器进行登录
if [ ! -s "$SESSION_FILE" ]; then
  echo "🔐 第一次登录，启动交互式 Telegram 登录流程..."
  touch "$SESSION_FILE"  # 创建空 session 文件以便挂载

  docker run --rm -it \
    -v "$CONFIG_FILE":/app/config.json:ro \
    -v "$SESSION_FILE":/app/session.json \
    -v /etc/localtime:/etc/localtime:ro \
    -e TZ=Asia/Shanghai \
    "$IMAGE_NAME"

  echo "✅ 登录完成，session.json 已保存。"
fi

# 启动守护容器
echo "🚀 启动后台服务..."
docker rm -f $APP_NAME 2>/dev/null || true

docker run -d --name $APP_NAME \
  -v "$CONFIG_FILE":/app/config.json:ro \
  -v "$SESSION_FILE":/app/session.json \
  -v /etc/localtime:/etc/localtime:ro \
  -e TZ=Asia/Shanghai \
  --restart=always \
  "$IMAGE_NAME"

echo "✅ 容器 $APP_NAME 已启动。"

