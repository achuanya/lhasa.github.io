---
layout: post
title: Docker
date: 2025-03-12 03:26:01 +0800
category: tech
thumb: ARTICLEPICTURES_PATH/golang.jpg
tags: [RSS, lhasaRSS, Go, 重构]
---

# wsl: 检测到 localhost 代理配置，但未镜像到 WSL。NAT 模式下的 WSL 不支持 localhost 代理。

# https://github.com/microsoft/WSL/releases/tag/2.0.0


```bash
.wslconfig

[experimental]
autoMemoryReclaim=gradual
networkingMode=mirrored
dnsTunneling=true
firewall=true
autoProxy=true

wsl --shutdown
wsl


&#10004; 环境初始化完成！
当前环境：
- Docker: Docker version 28.0.1, build 068a01e
- Node.js: v18.20.2
- ZSH: zsh 5.9 (x86_64-ubuntu-linux-gnu)

后续操作建议：
1. 重新打开终端或执行: source ~/.zshrc
2. 运行 p10k configure 配置 Powerlevel10k 主题
3. 如需 Docker 用户组生效，需退出并重新进入 WSL / 重新登录\033[0m
➜  lhasa   
```


docker pull ruby:3.2.2-slim-bullseye

docker tag ruby:3.2.2-slim-bullseye ruby:latest

安装 Docker Compose：


sudo curl -L "https://github.com/docker/compose/releases/download/1.29.2/docker-compose-$(uname -s)-$(uname -m)" -o /usr/local/bin/docker-compose
sudo chmod +x /usr/local/bin/docker-compose

