# banzi

把 iOS 发布流程包成命令行工具。编译成 Go 静态二进制，用 Homebrew 分发；
每个工具只干一件事、都自带 `help`、密钥只走环境变量不打印。

## 装

```sh
curl -fsSL https://raw.githubusercontent.com/douwantech/banzi/main/install.sh | sh
ship doctor
```

装到 `~/.local/bin`，无需 token。装完把 `~/.local/bin` 加进 PATH。

> Homebrew 也可以（`brew install douwantech/banzi/ship`），但 ship 是未签名二进制，
> cask 会打上 quarantine，首次运行要 Gatekeeper 联网核验——**墙内的机器会卡住**。
> 所以推荐上面的 curl 方式（curl 下载不带 quarantine，直接能跑）。等做了签名+公证再把 brew 转正。

## 工具

- **ship** —— iOS 上架。在 [`asc`](https://github.com/rorkai/App-Store-Connect-CLI)（App Store Connect CLI，Go、无 Ruby）
  之上套一层固定流程 + 护栏：`ship doctor | build | bump | certs | shots | beta | release | notarize`。
  跑在 GitHub Actions 的 macOS runner（按分钟、跑完就关），也能本地 Mac 跑。先 `ship doctor`。
  模板见 [`templates/ios/`](templates/ios/)（banzi.yaml + 可复用发布工作流）。设计见 [`CONVENTIONS.md`](CONVENTIONS.md)。
  需要 asc：`curl -fsSL https://asccli.sh/install | bash`。

## 发版

```sh
git tag vX.Y.Z && git push --tags
```
GoReleaser 编译全架构 → 建 GitHub Release → 自动更新 tap `douwantech/homebrew-banzi` 的 cask。

## 约定

- **不放任何密钥、账号、私有信息**：仓库公开，只放通用工具与占位配置。CI 里有密钥扫描拦截。
- 一个 App 一份 `banzi.yaml`，不同项目的 Apple team、bundle id、密钥各自隔离、互不混用。
