#!/bin/bash
set -e

# 加载 nvm 环境
if [ -s "$NVM_DIR/nvm.sh" ]; then
  . "$NVM_DIR/nvm.sh"
  nvm use $NODE_VERSION
fi

# 启动 yarn 和 Jekyll
yarn dev & 
exec bundle exec jekyll serve --host 0.0.0.0 --port 8888 --watch --force_polling