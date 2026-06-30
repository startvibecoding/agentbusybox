# AgentBusyBox

[![Go](https://img.shields.io/badge/Go-1.26-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)

一个完全使用 Go 编写的功能完备的 BusyBox 实现——单个静态二进制文件包含约 389 个 Unix/Linux 实用工具，内置容器根文件系统（rootfs）生成功能。

## 特性

- **单一二进制文件** — 一个自包含的可执行文件，内嵌约 389 个小程序（applet）
- **纯 Go 实现** — 不依赖任何外部系统命令（applet 实现中不使用 `exec.Command`）
- **多平台支持** — Linux（主要）、macOS、Windows
- **容器就绪** — 内置 rootfs 生成，支持 `--tar`、`--tar.gz` 和 `--minimal` 选项
- **Docker/Podman 兼容** — 生成可直接导入的 rootfs 镜像 tarball
- **软链接分发** — 支持软链接模式（传统 BusyBox 方式）和直接调用（`agentbusybox ls`）
- **Runit 支持** — 完整的 runit 进程监控套件
- **SELinux 工具** — 完整的 SELinux 策略管理
- **Shell** — 交互式 Shell，支持脚本执行

## 小程序分类

| 分类 | 说明 | 示例小程序 |
|------|------|-----------|
| `coreutils` | 核心 Unix 工具 | `ls`, `cat`, `cp`, `mv`, `rm`, `mkdir`, `echo`, `sort`, `wc`, `head`, `tail`, `cut`, `uniq`, `tr`, `sha256sum`, `md5sum` |
| `textproc` | 文本处理 | `grep`, `sed`, `diff`, `cmp`, `xargs` |
| `fileutil` | 文件操作 | `find`, `stat`, `du`, `df`, `ln`, `chmod`, `chown`, `tree`, `file`, `readlink`, `realpath` |
| `archival` | 归档与压缩 | `tar`, `gzip`, `bzip2`, `xz`, `zip`, `unzip`, `cpio`, `ar` |
| `networking` | 网络工具 | `wget`, `curl`, `ping`, `nc`, `ifconfig`, `ip`, `nslookup`, `netstat`, `telnet`, `httpd` |
| `process` | 进程管理 | `ps`, `top`, `kill`, `free`, `pgrep`, `pkill`, `lsof`, `pstree` |
| `shell` | Shell 与脚本 | `sh`, `ash`, `hush`, `printf`, `shuf` |
| `editors` | 文本编辑器 | `vi`, `ed`, `awk`, `patch` |
| `login` | 认证与用户 | `login`, `su`, `passwd`, `adduser`, `addgroup`, `mkpasswd` |
| `util-linux` | Linux 工具 | `mount`, `umount`, `fdisk`, `blkid`, `dmesg`, `flock`, `losetup`, `unshare` |
| `init` | 系统初始化 | `init`, `halt`, `reboot`, `poweroff`, `bootchartd` |
| `runit` | 服务监控 | `runsv`, `runsvdir`, `sv`, `chpst`, `svlogd` |
| `selinux` | SELinux 工具 | `chcon`, `getenforce`, `setenforce`, `restorecon`, `sestatus` |
| `misc` | 杂项工具 | `less`, `xxd`, `time`, `timeout`, `watch`, `nohup`, `bc`, `crond` |
| `fastutil` | 快速文件搜索 | `fd`（基于 go-fd / go-ripgrep） |
| `modutils` | 内核模块 | `insmod`, `lsmod`, `rmmod`, `modprobe`, `modinfo` |
| `sysklogd` | 日志系统 | `syslogd`, `klogd`, `logger`, `logread` |
| `debianutils` | Debian 工具 | `run-parts`, `start-stop-daemon` |
| `console` | 控制台控制 | `clear`, `reset`, `chvt`, `setfont`, `openvt` |
| `e2fsprogs` | Ext 文件系统 | `chattr`, `lsattr`, `tune2fs` |
| `mailutils` | 邮件工具 | `makemime`, `reformime`, `sendmail`, `popmaildir` |
| `printutils` | 打印工具 | `lpd`, `lpq`, `lpr` |
| `klibc` | klibc 工具 | `nuke`, `resume`, `run-init` |
| `rootfs` | 根文件系统生成 | `rootfs`（生成容器根文件系统） |

## 快速开始

### 编译

```bash
# 克隆仓库
git clone https://github.com/startvibecoding/agentbusybox.git
cd agentbusybox

# 为当前平台编译
make build

# 为所有平台交叉编译
make build-all
```

### 使用方法

```bash
# 直接调用
./bin/agentbusybox ls -la /tmp

# 软链接模式
ln -s ./bin/agentbusybox /usr/local/bin/ls
ls -la

# 列出所有小程序
./bin/agentbusybox --list

# 获取帮助
./bin/agentbusybox --help
```

### 容器 Rootfs

为容器生成最小根文件系统：

```bash
# 生成为目录
./bin/agentbusybox rootfs ./rootfs

# 生成为压缩 tarball
./bin/agentbusybox rootfs rootfs.tar.gz --tar.gz

# 生成 Docker/Podman 可用的镜像
make build-docker-image

# 导入 Docker
docker import bin/rootfs.tar.gz agentbusybox:latest
docker run -it agentbusybox:latest /bin/sh

# 导入 Podman
podman import bin/rootfs.tar.gz agentbusybox:latest
podman run -it agentbusybox:latest /bin/sh
```

## 编译目标

| 目标 | 说明 |
|------|------|
| `make build` | 为当前平台编译 |
| `make build-all` | 交叉编译 Linux/macOS/Windows 全平台 |
| `make build-linux` | 编译 Linux 二进制文件（amd64, arm64） |
| `make build-darwin` | 编译 macOS 二进制文件（amd64, arm64） |
| `make build-windows` | 编译 Windows 二进制文件（amd64, arm64） |
| `make build-docker-image` | 为 Docker/Podman 生成 rootfs tarball |
| `make dist` | 构建所有分发包 |
| `make install` | 通过 `go install` 安装 |
| `make test` | 运行测试（含竞态检测） |
| `make lint` | 运行 golangci-lint 静态检查 |
| `make fmt` | 使用 gofmt/goimports 格式化代码 |
| `make applets` | 列出所有已注册的小程序 |
| `make clean` | 清理构建产物 |

## 架构设计

AgentBusyBox 采用插件式的**小程序注册表**模式：

1. 每个小程序在 `init()` 中通过 `applet.Register()` 注册自身
2. `main.go` 中的空白导入（blank import）在启动时触发注册
3. `applet.Dispatch()` 从 `os.Args[0]`（软链接模式）或 `os.Args[1]`（busybox 模式）解析小程序名称并执行

### 添加新小程序

```go
// 在 cmd/myutils/myapplet.go 中
package myutils

import "github.com/agentbusybox/pkg/applet"

func init() {
    applet.Register(&applet.Applet{
        Name:  "myapplet",
        Short: "我的小程序描述",
        Func:  runMyApplet,
    })
}

func runMyApplet(args []string) int {
    // args[0] 是小程序名称
    // 用户参数从 args[1:] 开始
    return 0
}
```

然后在 `main.go` 中添加空白导入：

```go
_ "github.com/agentbusybox/cmd/myutils"
```

## 设计原则

- **纯 Go 实现** — 所有小程序使用 Go 标准库（`os`、`syscall`、`net`、`crypto/*`、`compress/*` 等），而非调用外部系统命令
- **零外部依赖** — 仅依赖 `golang.org/x/sys` 以及两个间接依赖（`go-fd`、`go-ripgrep`）
- **跨平台** — 使用 `pkg/platform` 辅助函数和 `runtime.GOOS` 判断
- **小体积** — 使用 `-ldflags "-s -w"` 剥离调试信息，可选 UPX 压缩

## 项目结构

```
agentbusybox/
├── main.go                  # 入口文件，包含所有 cmd 包的空白导入
├── Makefile                 # 构建系统
├── cmd/                     # 按分类的小程序实现
│   ├── coreutils/           # ls, cat, cp, mv, rm, ...
│   ├── networking/          # wget, curl, ping, nc, ip, ...
│   ├── process/             # ps, top, kill, free, ...
│   ├── shell/               # sh, ash, hush
│   ├── archival/            # tar, gzip, zip, ...
│   ├── textproc/            # grep, sed, diff, ...
│   ├── fileutil/            # find, stat, du, df, ...
│   ├── util-linux/          # mount, fdisk, dmesg, ...
│   ├── init/                # init, reboot, halt, ...
│   ├── runit/               # runsv, sv, chpst, ...
│   ├── selinux/             # chcon, setenforce, ...
│   ├── rootfs/              # rootfs 生成
│   └── ...                  # 共 26 个分类
└── pkg/
    ├── applet/              # 核心注册表: Register(), Dispatch(), List()
    └── platform/            # 跨平台抽象层
```

## 许可证

MIT
