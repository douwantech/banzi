// ship -- iOS 上架命令行（banzi）。
//
// ship 是 asc（App Store Connect CLI, github.com/rorkai/App-Store-Connect-CLI）之上的
// 一层固定流程 + 项目配置 + 护栏：把标准上架步骤做成几条清楚的子命令，底层调 asc
// （asc 用 Go、覆盖 编译/归档/导出 + 签名 + TestFlight + 送审 + 公证，无 Ruby）。
//
// 设计照 CONVENTIONS.md：help 是唯一手册、每条命令自带 help、doctor/dry-run/version、
// 不交互不自助、密钥只走环境变量不打印。跑在 GitHub Actions 的 macOS runner（跑完就关），
// 也能本地 Mac 跑。深层参数看 `asc help`。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Version is stamped at build time: -ldflags "-X main.Version=x.y.z".
var Version = "0.0.0-dev"

// command is one home for a subcommand's facts: its name, who runs it, its
// one-line (first line, shown in the index) and its full help.
type command struct {
	name  string
	group string
	one   string // index line
	help  string // full help (shown by `ship <name> --help` / `ship help <name>`)
	run   func(args []string) int
}

var commands []*command

func reg(c *command) { commands = append(commands, c) }

func get(name string) *command {
	for _, c := range commands {
		if c.name == name {
			return c
		}
	}
	return nil
}

func main() {
	registerAll()
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Println(index())
		return
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			if args[1] == "all" {
				fmt.Println(manual())
				return
			}
			if c := get(args[1]); c != nil {
				fmt.Println(c.help)
				return
			}
			fmt.Println(topic(args[1]))
			return
		}
		fmt.Println(index())
	case "version", "-v", "--version":
		fmt.Println(Version)
	default:
		c := get(args[0])
		if c == nil {
			jsonErr(fmt.Sprintf("未知命令: %s", args[0]), "跑 `ship help` 看命令列表")
			os.Exit(2)
		}
		rest := args[1:]
		if len(rest) > 0 && (rest[0] == "--help" || rest[0] == "-h") {
			fmt.Println(c.help)
			return
		}
		os.Exit(c.run(rest))
	}
}

// --- output helpers (CONVENTIONS: one JSON object; failure is {"error"}+nonzero) ---

func jsonErr(msg, fix string) {
	// minimal hand-rolled JSON to keep deps at zero
	esc := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		return s
	}
	fmt.Fprintf(os.Stderr, `{"error":"%s","fix":"%s"}`+"\n", esc(msg), esc(fix))
}

var dryRun = os.Getenv("SHIP_DRY") == "1"

// execAsc runs `asc <args...>` with passthrough stdio. A variable so tests can
// record the calls instead of running asc.
var execAsc = func(args []string) int {
	if _, err := exec.LookPath("asc"); err != nil {
		jsonErr("没找到 asc（App Store Connect CLI）", "装：curl -fsSL https://asccli.sh/install | bash  或 brew install asc；再 `ship doctor`")
		return 1
	}
	cmd := exec.Command("asc", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		jsonErr("asc 执行失败: "+err.Error(), "看上面的 asc 输出")
		return 1
	}
	return 0
}

// takeDry removes --dry from the arguments and says whether the command is a dry run
// (--dry anywhere, or SHIP_DRY=1).
func takeDry(args []string) ([]string, bool) {
	out := make([]string, 0, len(args))
	dry := dryRun
	for _, a := range args {
		if a == "--dry" {
			dry = true
			continue
		}
		out = append(out, a)
	}
	return out, dry
}

// asc runs one asc command; a dry run prints it instead.
func asc(args []string) int {
	out, dry := takeDry(args)
	return steps(dry, out)
}

// steps runs asc commands in order and stops at the first one that fails, so a later
// step never works on what an earlier one did not produce. A dry run prints them all.
func steps(dry bool, commands ...[]string) int {
	for _, c := range commands {
		if dry {
			fmt.Printf("asc %s\n", strings.Join(c, " "))
			continue
		}
		if rc := execAsc(c); rc != 0 {
			return rc
		}
	}
	return 0
}

// flagValue reads `--name value` or `--name=value` from passthrough arguments.
func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, name+"=") {
			return strings.TrimPrefix(a, name+"=")
		}
	}
	return ""
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name || strings.HasPrefix(a, name+"=") {
			return true
		}
	}
	return false
}

// splitAt cuts the arguments at the first bare `--`: what comes before, what comes after.
func splitAt(args []string) ([]string, []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// appID resolves the App Store Connect app id from $ASC_APP_ID or ./banzi.yaml.
func appID() (string, bool) {
	if v := os.Getenv("ASC_APP_ID"); v != "" {
		return v, true
	}
	b, err := os.ReadFile("banzi.yaml")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "asc_app_id:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "asc_app_id:"))
			v = strings.Trim(v, `"'`)
			if v != "" {
				return v, true
			}
		}
	}
	return "", false
}

func needApp() (string, bool) {
	id, ok := appID()
	if !ok {
		jsonErr("缺 App Store Connect app id", "在 banzi.yaml 写 `asc_app_id: \"123456789\"`，或设环境变量 ASC_APP_ID")
	}
	return id, ok
}

func registerAll() {
	commands = nil
	reg(&command{
		name: "doctor", group: "自检", one: "自检：asc / Xcode / 配置 / 鉴权齐不齐（只读，不改）",
		help: "# ship doctor\n\n只读检查环境：asc 是否安装、`asc xcode doctor`（Xcode 工具链）、`asc account status`（ASC 鉴权和账号状态）、当前目录有没有 banzi.yaml。改不动任何东西，每项给出修法。",
		run: func(a []string) int {
			rc := 0
			if _, err := exec.LookPath("asc"); err != nil {
				jsonErr("asc 未安装", "curl -fsSL https://asccli.sh/install | bash")
				rc = 1
			} else {
				fmt.Println("✓ asc 已安装")
				asc([]string{"xcode", "doctor"})
				asc([]string{"account", "status"})
			}
			if _, err := os.Stat("banzi.yaml"); err == nil {
				fmt.Println("✓ banzi.yaml 存在")
			} else {
				fmt.Println("· 当前目录没有 banzi.yaml（在项目目录里跑，或用 templates/ios 起一份）")
			}
			return rc
		},
	})
	reg(&command{
		name: "build", group: "构建", one: "归档并导出 .ipa（asc xcode archive + export）",
		help: "# ship build --scheme <scheme> [归档参数...] [-- 导出参数...]\n\n归档并导出 .ipa，两步：`asc xcode archive` 生成 build/<scheme>.xcarchive，再 `asc xcode export` 导出 build/<scheme>.ipa；归档失败就不导出。`--` 之前的参数给归档（--workspace/--project、--configuration、--clean 等），之后的给导出（--signing-style manual、--team-id、--export-options 等）。工程用手动签名时导出要加 `-- --signing-style manual`。只在本机出包，不上传。加 --dry 只打印将执行的两条命令。\n\n## 例\n`ship build --workspace App.xcworkspace --scheme App --configuration Release -- --signing-style manual`\n\n## 深层参数\n`asc xcode archive --help`、`asc xcode export --help`。",
		run: func(a []string) int {
			a, dry := takeDry(a)
			archiveArgs, exportArgs := splitAt(a)
			name := flagValue(archiveArgs, "--scheme")
			if name == "" {
				jsonErr("缺 --scheme", "例：ship build --scheme App；工程用手动签名时再加 `-- --signing-style manual`")
				return 2
			}
			archive, ipa := "build/"+name+".xcarchive", "build/"+name+".ipa"
			return steps(dry,
				append([]string{"xcode", "archive", "--archive-path", archive, "--overwrite"}, archiveArgs...),
				append([]string{"xcode", "export", "--archive-path", archive, "--ipa-path", ipa, "--overwrite"}, exportArgs...))
		},
	})
	reg(&command{
		name: "bump", group: "构建", one: "设版本号/构建号（asc xcode version edit）",
		help: "# ship bump <version> <build>  |  ship bump --next\n\n写版本号和构建号，底层 `asc xcode version edit --version <v> --build-number <b>`。`ship bump --next` 不改版本号，只把构建号设成 App Store Connect 上现有最大构建号的下一个（`--next-build-number`，要 ASC 鉴权和 app id）：仓库里记的构建号可能落后于 TestFlight，按后台的来才不会撞号。改的是工程文件，记得提交。额外参数透传（如 --project、--target）。",
		run: func(a []string) int {
			if len(a) >= 1 && a[0] == "--next" {
				id, ok := needApp()
				if !ok {
					return 2
				}
				return asc(append([]string{"xcode", "version", "edit", "--next-build-number", "--app", id}, a[1:]...))
			}
			if len(a) < 2 {
				jsonErr("用法: ship bump <version> <build>  或  ship bump --next", "例：ship bump 1.2.3 48；或 ship bump --next 取后台最大构建号的下一个")
				return 2
			}
			return asc(append([]string{"xcode", "version", "edit", "--version", a[0], "--build-number", a[1]}, a[2:]...))
		},
	})
	reg(&command{
		name: "certs", group: "签名", one: "同步签名证书 / 描述文件（asc signing）",
		help: "# ship certs\n\n管理签名证书与描述文件，底层 `asc signing`。证书私钥用环境变量/secret 注入，ship 不碰明文。额外参数透传。\n\n## 深层参数\n`asc signing --help`、`asc certificates --help`、`asc profiles --help`。",
		run:  func(a []string) int { return asc(append([]string{"signing"}, a...)) },
	})
	reg(&command{
		name: "shots", group: "素材", one: "本地截图 + 套壳 + 上传（asc screenshots）",
		help: "# ship shots\n\n本地截图、套机型壳、生成矩阵并上传，底层 `asc screenshots`。额外参数透传。\n\n## 深层参数\n`asc screenshots --help`。",
		run:  func(a []string) int { return asc(append([]string{"screenshots"}, a...)) },
	})
	reg(&command{
		name: "check", group: "发布", one: "上传前让苹果校验一遍 .ipa，不上传（asc xcode validate）",
		help: "# ship check <ipa>\n\n把 .ipa 交给苹果做上传前校验，底层 `asc xcode validate --ipa <ipa>`。只校验，不上传，不占构建号；通过了再去问老板要不要上传。鉴权走环境变量。",
		run: func(a []string) int {
			if len(a) < 1 {
				jsonErr("用法: ship check <ipa>", "例：ship check build/App.ipa")
				return 2
			}
			return asc(append([]string{"xcode", "validate", "--ipa", a[0]}, a[1:]...))
		},
	})
	reg(&command{
		name: "beta", group: "发布", one: "传 TestFlight，默认只上传（asc publish testflight）",
		help: "# ship beta <ipa> [--group <组> ...]\n\n把一个 .ipa 传到 TestFlight，底层 `asc publish testflight --app <id> --ipa <ipa>`。默认带 `--upload-only`：只上传，不进任何测试组、不通知测试员、不送 Beta 审核。要分发时自己加 `--group <组名或 id>`（这时不再带 --upload-only），通知和送审还要另加 --notify、--submit --confirm。app id 来自 banzi.yaml 的 asc_app_id 或环境变量 ASC_APP_ID。鉴权（ASC API Key）走环境变量，ship 不打印。传上去的构建删不掉，只能设为过期：上传前先得到老板同意。",
		run: func(a []string) int {
			if len(a) < 1 {
				jsonErr("用法: ship beta <ipa>", "例：ship beta build/App.ipa")
				return 2
			}
			id, ok := needApp()
			if !ok {
				return 2
			}
			args := []string{"publish", "testflight", "--app", id, "--ipa", a[0]}
			if !hasFlag(a[1:], "--group") && !hasFlag(a[1:], "--upload-only") {
				args = append(args, "--upload-only")
			}
			return asc(append(args, a[1:]...))
		},
	})
	reg(&command{
		name: "release", group: "发布", one: "上传并挂到 App Store 版本上，默认不送审（asc publish appstore）",
		help: "# ship release <ipa> <version> [--submit --confirm]\n\n上传 .ipa，找到或新建这个版本，把构建挂上去，底层 `asc publish appstore --app <id> --ipa <ipa> --version <version>`。默认到此为止，不送审。要送审自己加 `--submit --confirm`：送审和之后的上架都要老板逐次同意，不要替老板加。加 --dry 只打印命令；`--dry-run` 透传给 asc，由它预览整个计划而不上传。",
		run: func(a []string) int {
			if len(a) < 2 {
				jsonErr("用法: ship release <ipa> <version>", "例：ship release build/App.ipa 1.2.3")
				return 2
			}
			id, ok := needApp()
			if !ok {
				return 2
			}
			return asc(append([]string{"publish", "appstore", "--app", id, "--ipa", a[0], "--version", a[1]}, a[2:]...))
		},
	})
	reg(&command{
		name: "notarize", group: "签名", one: "提交公证并 staple 一个 Mac 产物（asc notarization submit + staple）",
		help: "# ship notarize <artifact>\n\n对一个已签名的 macOS 产物（.dmg 或 .pkg）提交苹果公证并等结果，通过后把票据 staple 上去，两步：`asc notarization submit --file <artifact> --wait`，再 `asc notarization staple --file <artifact> --confirm`；公证没过就不 staple。.zip 能公证但不能 staple，裸的命令行二进制两样都做不了（要公证得先打进 .zip/.dmg/.pkg）。额外参数透传给 submit（如 --timeout）。",
		run: func(a []string) int {
			a, dry := takeDry(a)
			if len(a) < 1 {
				jsonErr("用法: ship notarize <artifact>", "例：ship notarize dist/App.pkg")
				return 2
			}
			return steps(dry,
				append([]string{"notarization", "submit", "--file", a[0], "--wait"}, a[1:]...),
				[]string{"notarization", "staple", "--file", a[0], "--confirm"})
		},
	})
}

// --- help assembly (CONVENTIONS: index / per-command / topics / all) ---

func index() string {
	var b strings.Builder
	b.WriteString("ship —— iOS 上架命令行（banzi），asc 之上的固定流程 + 护栏\n\n")
	b.WriteString("【先读】\n")
	b.WriteString("- ship 把标准上架步骤包成子命令，底层调 asc（App Store Connect CLI）。先 `ship doctor`。\n")
	b.WriteString("- 密钥（ASC API Key、签名证书）只走环境变量，ship 不打印、不写日志。CI 放 GitHub Actions 加密 secret。\n")
	b.WriteString("- 不交互、不自助：缺东西就报错并给出修复那一行；加 --dry 只打印将执行的 asc 命令。\n")
	b.WriteString("- app id 来自项目目录的 banzi.yaml（asc_app_id）或环境变量 ASC_APP_ID。深层参数看 `asc help`。\n\n")
	groups := []string{"自检", "构建", "签名", "素材", "发布"}
	for _, g := range groups {
		first := true
		for _, c := range commands {
			if c.group != g {
				continue
			}
			if first {
				b.WriteString("【" + g + "】\n")
				first = false
			}
			b.WriteString(fmt.Sprintf("  ship %-10s %s\n", c.name, c.one))
		}
	}
	b.WriteString("  ship help <命令|all>   看某条/全部手册\n")
	b.WriteString("  ship version\n\n")
	b.WriteString("【退出码】0 成功；2 用法错误；其它非零=失败（asc 的退出码原样透出）。\n")
	b.WriteString("【安装】curl -fsSL https://raw.githubusercontent.com/douwantech/banzi/main/install.sh | sh\n")
	return b.String()
}

func manual() string {
	var b strings.Builder
	b.WriteString(index())
	b.WriteString("\n========== 全部命令手册 ==========\n")
	for _, c := range commands {
		b.WriteString("\n" + c.help + "\n")
	}
	b.WriteString("\n" + topic("config") + "\n")
	return b.String()
}

func topic(name string) string {
	switch name {
	case "config":
		return "# 配置与密钥（topic）\n\n一个 App 一份 banzi.yaml（asc_app_id、公司/项目代号等），不同项目各自隔离、绝不混用。密钥不写进 banzi.yaml，只走环境变量，名字是 asc 认的这几个：ASC_KEY_ID、ASC_ISSUER_ID，加上私钥三选一：ASC_PRIVATE_KEY（.p8 的文本）、ASC_PRIVATE_KEY_B64（.p8 的 base64）、ASC_PRIVATE_KEY_PATH（.p8 的路径）。CI 放 GitHub Actions 加密 secret，本地用你的密钥管理工具注入到环境变量后再跑 ship。只想查不想改时设 ASC_READ_ONLY=1，asc 会拒绝一切写操作。"
	default:
		return fmt.Sprintf(`{"error":"没有这个命令或 topic: %s","fix":"ship help 看列表"}`, name)
	}
}
