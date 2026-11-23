# banzi

把 iOS 发布流程包成命令行工具。编译成 Go 静态二进制，用 Homebrew 分发；
每个工具只干一件事、都自带 `help`、密钥只走环境变量不打印。

## 装

```sh
curl -fsSL https://raw.githubusercontent.com/douwantech/banzi/main/install.sh | sh
ship doctor
```

装到 `~/.local/bin`，无需 token。装完把 `~/.local/bin` 加进 PATH。

> Homebrew 也可以（`brew install douwantech/banzi/ship`）。cask 装的文件带 quarantine 标记，首次运行时
> Gatekeeper 要联网向苹果核对公证记录（命令行二进制没法把公证票据随文件带上）——**网络受限的机器
> 首次运行仍可能卡住**，这种机器用上面的 curl 方式（curl 下载不带 quarantine，直接能跑）。
> v0.2.0 及更早的版本没有签名，用 brew 装了跑不起来；之后的版本带 Developer ID 签名并经过公证。

## 工具

- **ship** —— iOS 上架。在 [`asc`](https://github.com/rorkai/App-Store-Connect-CLI)（App Store Connect CLI，Go、无 Ruby）
  之上套一层固定流程 + 护栏：`ship doctor | build | bump | certs | shots | check | beta | release | notarize`。
  跑在 GitHub Actions 的 macOS runner（按分钟、跑完就关），也能本地 Mac 跑。先 `ship doctor`。
  模板见 [`templates/ios/`](templates/ios/)（banzi.yaml + 可复用发布工作流）。设计见 [`CONVENTIONS.md`](CONVENTIONS.md)。
  需要 asc：`curl -fsSL https://asccli.sh/install | bash`。

## 发版

```sh
git tag vX.Y.Z && git push --tags
```
tag 推上去后由 [`.github/workflows/release.yml`](.github/workflows/release.yml) 接手：测试 → GoReleaser 编译全架构 →
用 Developer ID 证书给 macOS 二进制签名并提交苹果公证 → 建 GitHub Release → 更新 tap `douwantech/homebrew-banzi` 的 cask。

证书和密钥不进仓库，放在仓库的 `release` 环境里（Settings → Environments → release）。建议给这个环境设
「只允许 `v*` tag 部署」和「需要批准」，这样只有发版任务读得到，每次发版有人点头。

| secret | 内容 |
|---|---|
| `MACOS_SIGN_P12` | Developer ID Application 证书（含私钥）导出的 .p12，base64 |
| `MACOS_SIGN_PASSWORD` | 这个 .p12 的密码 |
| `MACOS_NOTARY_ISSUER_ID`、`MACOS_NOTARY_KEY_ID`、`MACOS_NOTARY_KEY` | 公证用的 App Store Connect API 密钥：Issuer ID、Key ID、.p8 的 base64（Developer 角色就够） |
| `HOMEBREW_TAP_DEPLOY_KEY` | tap 仓库的一把可写部署密钥的私钥（本仓库自带的 token 写不了别的仓库；部署密钥只能动 tap 这一个仓库） |

Developer ID 证书只有 Apple 开发者账号的持有人（Account Holder）能建，App Store Connect API 密钥建不了。

也可以在本机发版，证书不出那台机器：把上面几个变量设成本机文件的路径，再跑
`GITHUB_TOKEN=… goreleaser release --clean`。没设 `MACOS_SIGN_P12` 时签名公证这步会跳过，
只适合 `goreleaser release --snapshot --clean` 试编译；正式发版的工作流最后会核对二进制确实带签名，没带就失败。

## 约定

- **不放任何密钥、账号、私有信息**：仓库公开，只放通用工具与占位配置。CI 里有密钥扫描拦截。
- 一个 App 一份 `banzi.yaml`，不同项目的 Apple team、bundle id、密钥各自隔离、互不混用。
