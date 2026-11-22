#!/bin/sh
# banzi · ship 安装器。公开仓库，无需 token：
#   curl -fsSL https://raw.githubusercontent.com/douwantech/banzi/main/install.sh | sh
#
# 为什么用 curl 而不是 Homebrew：ship 是未签名的 Go 二进制；Homebrew cask 会给它打上
# quarantine/provenance，首次运行时 macOS Gatekeeper 要联网核验，墙内的机器会卡住。
# curl 下载的二进制不带这些标记，直接能跑。装到 ~/.local/bin，不碰系统目录。
set -eu

REPO="douwantech/banzi"
os=$(uname -s | tr 'A-Z' 'a-z')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "不支持的 CPU 架构：$arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin|linux) ;;
  *) echo "只支持 macOS / Linux（当前 $os）" >&2; exit 1 ;;
esac

api="https://api.github.com/repos/$REPO/releases/latest"
json=$(curl -fsSL "$api")
url=$(printf '%s' "$json" | grep -o "https://[^\"]*ship_[^\"]*_${os}_${arch}\.tar\.gz" | head -1)
[ -n "$url" ] || { echo "最新 Release 里没有 ${os}/${arch} 的 ship 包" >&2; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
curl -fsSL "$url" -o "$tmp/ship.tgz"

# 校验 sha256（对 checksums.txt）
sumurl=$(printf '%s' "$json" | grep -o "https://[^\"]*checksums\.txt" | head -1)
if [ -n "$sumurl" ]; then
  curl -fsSL "$sumurl" -o "$tmp/sums.txt"
  fname=$(basename "$url")
  want=$(grep "  *$fname\$" "$tmp/sums.txt" | awk '{print $1}' | head -1 || true)
  if [ -n "$want" ]; then
    if command -v sha256sum >/dev/null 2>&1; then got=$(sha256sum "$tmp/ship.tgz" | awk '{print $1}')
    else got=$(shasum -a 256 "$tmp/ship.tgz" | awk '{print $1}'); fi
    [ "$got" = "$want" ] || { echo "校验不对，没有安装" >&2; exit 1; }
  fi
fi

tar xzf "$tmp/ship.tgz" -C "$tmp"
dir="$HOME/.local/bin"
mkdir -p "$dir"
install -m 0755 "$tmp/ship" "$dir/ship"
echo "已安装：$dir/ship"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) echo "提示：把 $dir 加进 PATH（例如 echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc）" ;;
esac
"$dir/ship" version
