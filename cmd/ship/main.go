// ship -- iOS 上架命令行（banzi）。
//
// 它不重写 Fastlane——Fastlane 本身就是 iOS 发布的引擎。ship 在它上面套一层
// 固定流程 + 护栏：把 Fastlane 的 iOS 全家桶包成几条清楚的子命令（certs/build/
// beta/release/shots/bump/app-create），读项目里的 banzi.yaml / 环境变量，调对应
// 的 Fastlane lane。密钥只走环境变量，不打印。装上后 `ship help` 看用法。
//
// 典型跑在 GitHub Actions 的 macOS runner 上（跑完就关），也能在本地 Mac 跑。
package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Version is stamped at build time: -ldflags "-X main.Version=x.y.z".
var Version = "0.0.0-dev"

// lanes maps a ship subcommand to the Fastlane iOS lane it runs, with a one-line 说明.
var lanes = []struct{ cmd, lane, desc string }{
	{"certs", "certs", "证书/描述文件（fastlane match，加密存一处、团队共用）"},
	{"build", "build", "出包（gym → .ipa）"},
	{"beta", "beta", "传 TestFlight（pilot）"},
	{"release", "release", "元数据+截图+二进制 上架送审（deliver + precheck）"},
	{"shots", "shots", "自动截图（snapshot + frameit）"},
	{"bump", "bump", "版本号 / build 号自增"},
	{"app-create", "app_create", "在 App Store Connect 新建 App（produce）"},
}

const helpHead = `ship —— iOS 上架命令行（班子 banzi）

【先读】
- ship 把 Fastlane 的 iOS 功能包成固定流程 + 护栏。真正干活的引擎是 Fastlane，
  ship 负责：统一子命令、读配置、传对密钥、守规矩。忘了用法随时 ` + "`ship help`" + `。
- 密钥（App Store Connect API Key、match 密码、签名证书）只走环境变量，ship 不打印、
  不写进日志或文件。CI 里放 GitHub Actions 加密 secret；本地用你的密钥管理工具注入。
- 一个项目一份 banzi.yaml（放 bundle id、Apple team、ASC key 的来源等），多个项目
  各自隔离、绝不混用。
`

func helpText() string {
	var b strings.Builder
	b.WriteString(helpHead)
	b.WriteString("\n【命令】\n")
	b.WriteString("  ship doctor             自检：Xcode / Ruby / Fastlane / 配置齐不齐\n")
	for _, l := range lanes {
		b.WriteString(fmt.Sprintf("  ship %-18s %s\n", l.cmd, l.desc))
	}
	b.WriteString("  ship help / version\n")
	b.WriteString(`
  每个 lane 子命令 = 跑一条标准化的 Fastlane lane（fastlane ios <lane>），
  后面可以带额外参数透传给 Fastlane，例如 ship build -- --verbose。

【安装】
  brew install douwantech/banzi/ship     # 装
  ship doctor                            # 装完先自检

【退出码】0 成功；2 用法错误；其它非零=失败（Fastlane 的退出码会原样透出）。`)
	return b.String()
}

// runFastlane execs `fastlane ios <lane>` with passthrough args, inheriting stdio.
func runFastlane(lane string, extra []string) int {
	if _, err := exec.LookPath("fastlane"); err != nil {
		fmt.Fprintln(os.Stderr, "ship: 没找到 fastlane。先装：brew install fastlane（或 bundle）。`ship doctor` 可自检。")
		return 1
	}
	args := append([]string{"ios", lane}, extra...)
	cmd := exec.Command("fastlane", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "ship:", err)
		return 1
	}
	return 0
}

func doctor() int {
	type chk struct {
		name string
		args []string
	}
	checks := []chk{
		{"xcodebuild", []string{"-version"}},
		{"xcode-select", []string{"-p"}},
		{"ruby", []string{"-v"}},
		{"fastlane", []string{"--version"}},
	}
	ok := true
	for _, c := range checks {
		if _, err := exec.LookPath(c.name); err != nil {
			fmt.Printf("✗ %-13s 没装\n", c.name)
			ok = false
			continue
		}
		out, _ := exec.Command(c.name, c.args...).CombinedOutput()
		first := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
		fmt.Printf("✓ %-13s %s\n", c.name, first)
	}
	if _, err := os.Stat("banzi.yaml"); err == nil {
		fmt.Println("✓ banzi.yaml    当前目录有项目配置")
	} else {
		fmt.Println("· banzi.yaml    当前目录没有（跑某个项目时在它的目录里，或用模板 templates/ios 起一份）")
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "\n有缺的：iOS 构建要 macOS + Xcode + Ruby + Fastlane。")
		return 1
	}
	fmt.Println("\n环境齐了。")
	return 0
}

func main() {
	args := os.Args[1:]
	cmd := "help"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "help", "-h", "--help":
		fmt.Println(helpText())
	case "version", "-v", "--version":
		fmt.Println(Version)
	case "doctor":
		os.Exit(doctor())
	default:
		for _, l := range lanes {
			if l.cmd == cmd {
				extra := args[1:]
				// allow `ship build -- --flag` passthrough
				if len(extra) > 0 && extra[0] == "--" {
					extra = extra[1:]
				}
				os.Exit(runFastlane(l.lane, extra))
			}
		}
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		fmt.Println(helpText())
		os.Exit(2)
	}
}
