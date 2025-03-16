#!/usr/bin/env bash
set -e

############################################
#       WSL + Ubuntu 环境初始化脚本        #
#  适用于国内网络环境 + zsh + nvm + 回滚   #
############################################

# ====== 配置参数 ======
INSTALL_DOCKER=true          # 是否安装Docker
CHANGE_APT_SOURCE=true       # 是否更换APT源
UBUNTU_MIRROR_URL="https://mirrors.tuna.tsinghua.edu.cn"
YOUR_NAME="achuanya"
YOUR_EMAIL="haibao1027@gmail.com"
USE_PROXY=false              # 是否启用代理
PROXY_ADDR="http://127.0.0.1:10807" # 代理地址
DOMAIN="ghfast.top" # Docker Proxy 经常被封

# ====== 预配置 ======
# 修复Windows换行符问题（若当前脚本本地保存时有CRLF问题）
sed -i 's/\r$//' "$0"

# 根据需要配置网络代理
if [ "$USE_PROXY" = true ]; then
  export http_proxy="$PROXY_ADDR"
  export https_proxy="$PROXY_ADDR"
  git config --global http.proxy "$PROXY_ADDR"
  git config --global https.proxy "$PROXY_ADDR"
fi

# ====== 系统基础配置 ======
# 配置时区和语言
sudo timedatectl set-timezone Asia/Shanghai
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y locales
sudo sed -i '/en_US.UTF-8/s/^# //' /etc/locale.gen
sudo locale-gen en_US.UTF-8 > /dev/null
sudo update-locale LANG=en_US.UTF-8 > /dev/null

# ====== APT源配置（含自动回滚） ======
if [ "$CHANGE_APT_SOURCE" = true ]; then
  # 备份当前 sources.list
  sudo cp /etc/apt/sources.list /etc/apt/sources.list.bak || true

  VERSION_CODENAME=$(lsb_release -sc)
  sudo tee /etc/apt/sources.list > /dev/null << EOF
deb ${UBUNTU_MIRROR_URL}/ubuntu/ ${VERSION_CODENAME} main restricted universe multiverse
deb ${UBUNTU_MIRROR_URL}/ubuntu/ ${VERSION_CODENAME}-updates main restricted universe multiverse
deb ${UBUNTU_MIRROR_URL}/ubuntu/ ${VERSION_CODENAME}-backports main restricted universe multiverse
deb ${UBUNTU_MIRROR_URL}/ubuntu/ ${VERSION_CODENAME}-security main restricted universe multiverse
EOF
fi

# ====== 工具安装：带指数退避重试的函数 ======
retry_apt() {
  local retries=3 count=0
  local cmd=("sudo" "apt-get" "-o" "Acquire::Retries=3" "$@")

  until [ $count -ge $retries ]; do
    if "${cmd[@]}"; then
      return 0
    else
      count=$((count + 1))
      echo "重试 apt 操作 (#$count)..."
      sleep $((2**count))  # 简单指数退避
    fi
  done

  echo "apt 操作失败: ${cmd[*]}" >&2
  return 1
}

# 首次 apt update，如失败且启用了换源，自动回滚一次
if ! retry_apt update; then
  if [ "$CHANGE_APT_SOURCE" = true ]; then
    echo "APT update 失败，尝试回滚 sources.list ..."
    sudo cp /etc/apt/sources.list.bak /etc/apt/sources.list || true
    if ! retry_apt update; then
      echo "APT update 在回滚后依旧失败，请检查网络。" >&2
      exit 1
    fi
  else
    echo "APT update 失败，请检查网络。" >&2
    exit 1
  fi
fi

# 继续升级系统
retry_apt upgrade -y

# 安装常用工具
retry_apt install -y \
  zsh git curl wget build-essential \
  dnsutils lsof python3-pip ssh \
  neovim tmux htop jq ripgrep \
  fzf bat eza duf tldr glances \
  vim net-tools iproute2 dos2unix \
  ca-certificates gnupg mtr traceroute

# ====== Docker配置 ======
if [ "$INSTALL_DOCKER" = true ]; then
  # 检查systemd可用性（WSL内默认并不是systemd，但用户可额外启用）
  if ! systemctl list-units --type=service | grep -q systemd-journald; then
    echo -e "\033[33m  检测到systemd不可用，Docker可能无法正常运行\033[0m"
    read -p "是否继续安装？(y/n) " -n 1 -r
    echo
    [[ $REPLY =~ ^[Yy]$ ]] || exit 1
  fi

  # 添加阿里云Docker源
  sudo install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://mirrors.aliyun.com/docker-ce/linux/ubuntu/gpg \
    | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
  https://mirrors.aliyun.com/docker-ce/linux/ubuntu \
  $(lsb_release -cs) stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

  retry_apt update
  retry_apt install -y docker-ce docker-ce-cli containerd.io \
    docker-buildx-plugin docker-compose-plugin

  # 配置用户组和镜像加速
  sudo usermod -aG docker "$USER"
  sudo mkdir -p /etc/docker
  sudo tee /etc/docker/daemon.json > /dev/null << EOF
{
  "registry-mirrors": [
    "https://dockerproxy.com",
    "https://hub-mirror.c.163.com",
    "https://mirror.baidubce.com"
  ],
  "exec-opts": ["native.cgroupdriver=systemd"]
}
EOF
  sudo systemctl enable docker
  sudo systemctl restart docker
fi

# ====== Git配置 ======
git config --global user.name "$YOUR_NAME"
git config --global user.email "$YOUR_EMAIL"
git config --global core.editor nvim
git config --global pull.rebase true
git config --global http.postBuffer 1048576000
git config --global --add url.https://$DOMAIN/https://github.com.insteadOf https://github.com

# ====== 安装 nvm（多源重试） ======
install_nvm() {
  echo "开始安装 nvm..."
  local attempts=0
  while [ $attempts -lt 3 ]; do
    echo "尝试从 npmmirror 安装 nvm (第 $((attempts+1)) 次)..."
    if curl -kL https://$DOMAIN/https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash; then
      return 0
    fi
    echo "npmmirror 源安装失败，改用 Docker Proxy 源..."

    if curl -kL https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash; then
      return 0
    fi
    echo "Docker Proxy 源安装失败，改用 GitHub 官方源..."

    if curl -kL https://npmmirror.com/mirrors/nvm/v0.39.7/install.sh | bash; then
      return 0
    fi
    echo "GitHub 官方源安装也失败了，等待重试..."
    attempts=$((attempts + 1))
    sleep $((2**attempts))
  done

  echo "nvm 安装失败，请检查网络或手动安装。" >&2
  return 1
}

# 如果系统里还没有 nvm，就执行多源安装
export NVM_DIR="$HOME/.nvm"
if [ ! -s "$NVM_DIR/nvm.sh" ]; then
  install_nvm
fi

# 确保脚本内能加载 nvm
[ -s "$NVM_DIR/nvm.sh" ] && source "$NVM_DIR/nvm.sh"

# 在 .zshrc 中确保 nvm 可用
if ! grep -q 'export NVM_DIR="$HOME/.nvm"' ~/.zshrc 2>/dev/null; then
cat >> ~/.zshrc << 'EOF'
# Node.js配置
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
export NVM_NODEJS_ORG_MIRROR="https://registry.npmmirror.com/binary.html?path=node/"
EOF
fi

# 按需安装多个 Node.js 版本
if command -v nvm &>/dev/null; then
  nvm install 20.14.0 --lts || true
  nvm install 18.20.2 --lts || true
  nvm alias default 20.14.0 || true
fi

# ====== ZSH配置 ======
# 设置ZSH为默认shell
if [ "$(basename "$SHELL")" != "zsh" ]; then
  sudo chsh -s "$(which zsh)" "$USER"
fi

# 安装 Oh My Zsh（带镜像源和重试）
install_ohmyzsh() {
  echo "尝试通过镜像安装 Oh My Zsh..."
  local attempts=0
  until [ $attempts -ge 3 ]; do
    # 先用 gitee 镜像
    if sh -c "$(curl -kfsSL https://gitee.com/mirrors/oh-my-zsh/raw/master/tools/install.sh)" "" --unattended; then
      return 0
    fi
    attempts=$((attempts + 1))
    echo "安装失败，尝试备用源 (#$attempts)..."
    # 再用 GitHub（Docker Proxy）
    if sh -c "$(curl -kfsSL https://$DOMAIN/https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" "" --unattended; then
      return 0
    fi
    sleep 5
  done
  echo "Oh My Zsh 安装失败" >&2
  return 1
}

[ ! -d "$HOME/.oh-my-zsh" ] && install_ohmyzsh

# 安装常用插件
install_zsh_plugin() {
  local repo="$1"
  local name="$2"
  local folder_type="$3"
  local dest="${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/$folder_type/$name"

  if [ ! -d "$dest" ]; then
    git clone --depth=1 "https://$DOMAIN/https://github.com/$repo" "$dest"
  fi
}

install_zsh_plugin "zsh-users/zsh-autosuggestions" "zsh-autosuggestions" "plugins"
install_zsh_plugin "zsh-users/zsh-syntax-highlighting" "zsh-syntax-highlighting" "plugins"
install_zsh_plugin "romkatv/powerlevel10k" "powerlevel10k" "themes"

# 在 .zshrc 中追加自定义配置（如已有则不重复写入）
if ! grep -q 'alias ls=' ~/.zshrc 2>/dev/null; then
cat >> ~/.zshrc << 'EOF'
# ==== 自定义配置 ====
export PATH="$HOME/.local/bin:$HOME/.cargo/bin:$PATH"

# 别名配置
alias ll='eza -alF --git --icons --color=always'
alias ls='eza --icons --color=auto'
alias cat='bat --style=plain --paging=never'
alias dps='docker ps --format "table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}"'
alias vim='nvim'

# FZF配置
[ -f /usr/share/doc/fzf/examples/key-bindings.zsh ] && source /usr/share/doc/fzf/examples/key-bindings.zsh
[ -f /usr/share/doc/fzf/examples/completion.zsh ] && source /usr/share/doc/fzf/examples/completion.zsh

# Powerlevel10k主题
[[ ! -f ~/.p10k.zsh ]] || source ~/.p10k.zsh
EOF
fi

# ====== 清理和收尾 ======
retry_apt autoremove -y
retry_apt clean
rm -rf "$HOME/.cache/*"

# 修复WSL CUDA链接警告（忽略报错）
sudo ln -sf /usr/lib/wsl/lib/libcuda.so.1 /usr/lib/wsl/lib/libcuda.so 2>/dev/null || true
sudo ldconfig

# 完成提示
echo -e "\n\033[32m&#10004; 环境初始化完成！\033[0m"
echo -e "当前环境："
echo -e "- Docker: \033[33m$(docker --version 2>/dev/null || echo '未安装')\033[0m"
echo -e "- Node.js: \033[33m$(node --version 2>/dev/null || echo '未安装')\033[0m"
echo -e "- ZSH: \033[33m$(zsh --version)\033[0m"

echo -e "\n\033[36m后续操作建议："
echo "1. 重新打开终端或执行: source ~/.zshrc"
echo "2. 运行 p10k configure 配置 Powerlevel10k 主题"
echo "3. 如需 Docker 用户组生效，需退出并重新进入 WSL / 重新登录\033[0m"

# 脚本最后自动切换 zsh
exec zsh -l
