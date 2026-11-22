# banzi

把 iOS 发布流程包成命令行工具。编译成 Go 静态二进制，用 Homebrew 分发；
每个工具只干一件事、都自带 `help`、密钥只走环境变量不打印。

## 装

```sh
brew install douwantech/banzi/ship
ship doctor
```

## 工具

- **ship** —— iOS 上架。在 Fastlane 之上套一层固定流程 + 护栏：
  `ship doctor | certs | build | beta | release | shots | bump | app-create`。
  跑在 GitHub Actions 的 macOS runner（按分钟、跑完就关），也能本地 Mac 跑。
  模板见 [`templates/ios/`](templates/ios/)（Fastfile、Appfile、banzi.yaml、可复用发布工作流）。

## 发版

```sh
git tag vX.Y.Z && git push --tags
```
GoReleaser 编译全架构 → 建 GitHub Release → 自动更新 tap `douwantech/homebrew-banzi` 的 cask。

## 约定

- **不放任何密钥、账号、私有信息**：仓库公开，只放通用工具与占位配置。CI 里有密钥扫描拦截。
- 一个 App 一份 `banzi.yaml`，不同项目的 Apple team、bundle id、密钥各自隔离、互不混用。
