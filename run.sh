docker run -d --name telegram-env-watcher \
  -v ${PWD}/config.json:/app/config.json:ro \
  -v ${PWD}/session.json:/app/session.json \
  --restart=always \
  telegram-env-watcher:latest
