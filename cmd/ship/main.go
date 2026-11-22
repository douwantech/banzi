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

// asc builds and runs `asc <args...>` with passthrough stdio. With --dry (or
// SHIP_DRY=1) it prints the command instead of running it.
func asc(args []string) int {
	// honor a leading --dry on the subcommand
	out := make([]string, 0, len(args))
	dry := dryRun
	for _, a := range args {
		if a == "--dry" {
			dry = true
			continue
		}
		out = append(out, a)
	}
	if dry {
		fmt.Printf("asc %s\n", strings.Join(out, " "))
		return 0
	}
	if _, err := exec.LookPath("asc"); err != nil {
		jsonErr("没找到 asc（App Store Connect CLI）", "装：curl -fsSL https://asccli.sh/install | bash  或 brew install asc；再 `ship doctor`")
		return 1
	}
	cmd := exec.Command("asc", out...)
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
		help: "# ship doctor\n\n只读检查环境：asc 是否安装、`asc xcode doctor`（Xcode 工具链）、`asc account`（ASC 鉴权）、当前目录有没有 banzi.yaml。改不动任何东西，每项给出修法。",
		run: func(a []string) int {
			rc := 0
			if _, err := exec.LookPath("asc"); err != nil {
				jsonErr("asc 未安装", "curl -fsSL https://asccli.sh/install | bash")
				rc = 1
			} else {
				fmt.Println("✓ asc 已安装")
				asc([]string{"xcode", "doctor"})
				asc([]string{"account"})
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
		name: "build", group: "构建", one: "编译/归档/导出 .ipa（asc xcode build）",
		help: "# ship build\n\n编译并导出 .ipa，底层 `asc xcode build`。额外参数透传给 asc（如 --scheme、--configuration）。加 --dry 只打印将执行的命令。\n\n## 深层参数\n`asc xcode build --help`。",
		run:  func(a []string) int { return asc(append([]string{"xcode", "build"}, a...)) },
	})
	reg(&command{
		name: "bump", group: "构建", one: "设版本号/build 号（asc xcode inject）",
		help: "# ship bump <version> <build>\n\n写版本号和 build 号，底层 `asc xcode inject --set version=<v> --set build_number=<b>`。build 号建议用时间戳（稳不回退）。",
		run: func(a []string) int {
			if len(a) < 2 {
				jsonErr("用法: ship bump <version> <build>", "例：ship bump 1.2.3 202611221900")
				return 2
			}
			return asc(append([]string{"xcode", "inject", "--set", "version=" + a[0], "--set", "build_number=" + a[1]}, a[2:]...))
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
		name: "beta", group: "发布", one: "传 TestFlight（asc publish testflight）",
		help: "# ship beta <ipa>\n\n把一个 .ipa 推到 TestFlight，底层 `asc publish testflight --app <id> --ipa <ipa>`。app id 来自 banzi.yaml 的 asc_app_id 或环境变量 ASC_APP_ID。鉴权（ASC API Key）走环境变量，ship 不打印。",
		run: func(a []string) int {
			if len(a) < 1 {
				jsonErr("用法: ship beta <ipa>", "例：ship beta build/App.ipa")
				return 2
			}
			id, ok := needApp()
			if !ok {
				return 2
			}
			return asc(append([]string{"publish", "testflight", "--app", id, "--ipa", a[0]}, a[1:]...))
		},
	})
	reg(&command{
		name: "release", group: "发布", one: "元数据+截图+二进制 上架送审（asc publish appstore）",
		help: "# ship release <ipa> <version>\n\n上架送审：元数据/截图/二进制一起提交，底层 `asc publish appstore --app <id> --ipa <ipa> --version <version>`。送审前 asc 会做校验。可用 --dry 先看命令。",
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
		name: "notarize", group: "签名", one: "公证并 staple 一个 Mac 产物（asc notarization staple）",
		help: "# ship notarize <artifact>\n\n对一个已签名的 macOS 产物（.pkg/.dmg/.app）做公证并 staple，底层 `asc notarization staple <artifact>`（必要时 `asc notarization validate`）。这步也用来给 banzi 自己的工具签名发布。",
		run: func(a []string) int {
			if len(a) < 1 {
				jsonErr("用法: ship notarize <artifact>", "例：ship notarize dist/App.pkg")
				return 2
			}
			return asc(append([]string{"notarization", "staple"}, a...))
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
		return "# 配置与密钥（topic）\n\n一个 App 一份 banzi.yaml（asc_app_id、公司/项目代号等），不同项目各自隔离、绝不混用。密钥不写进 banzi.yaml：CI 放 GitHub Actions 加密 secret（ASC_KEY_ID/ASC_ISSUER_ID/ASC_KEY_P8 等），本地用你的密钥管理工具注入到环境变量后再跑 ship。"
	default:
		return fmt.Sprintf(`{"error":"没有这个命令或 topic: %s","fix":"ship help 看列表"}`, name)
	}
}
