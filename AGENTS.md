# AGENTS.md — AgentBusyBox

## Project Snapshot

- **Repo:** https://github.com/startvibecoding/agentbusybox.git
- **Language:** Go 1.26 (module `github.com/agentbusybox`)
- **Purpose:** A full-featured BusyBox implementation in Go — a single binary containing ~389 Unix/Linux utilities, including rootfs generation for containers. Invoked as `agentbusybox <applet> [args...]` or via symlinks.
- **Dependencies:** `golang.org/x/sys`, `github.com/startvibecoding/go-fd` (indirect), `github.com/startvibecoding/go-ripgrep` (indirect)
- **Platforms:** Linux (primary), macOS, Windows

## Important Directories

| Path | Contents |
|------|----------|
| `main.go` | Entry point. Imports all `cmd/` packages via blank imports, dispatches to applet registry. |
| `cmd/` | Applet implementations, one package per category. Each registers applets in `init()`. |
| `cmd/coreutils/` | Core utilities: `ls`, `cat`, `echo`, `cp`, `mv`, `rm`, `mkdir`, `touch`, `sort`, `uniq`, `tr`, `wc`, `head`, `tail`, `cut`, `comm`, `test`, `uname`, `id`, `install`, `sleep`, `tac`, `base64`, `base32`, `md5sum`, `sha256sum`, `sha512sum`, `nproc`, `printenv`, `expr`, `cksum`, `dos2unix`, `unix2dos`, `uuencode`, `uudecode`, `arch`, `who`, `w`, etc. |
| `cmd/textproc/` | Text processing: `grep`, `sed`, `diff`, `cmp`, `strings`, `xargs` |
| `cmd/fileutil/` | File utilities: `find`, `stat`, `du`, `df`, `ln`, `chmod`, `chown`, `chgrp`, `which`, `file`, `tree`, `mktemp`, `readlink`, `realpath` |
| `cmd/archival/` | Archive/compression: `tar`, `gzip`, `gunzip`, `bzip2`, `bunzip2`, `lzma`, `xz`, `cpio`, `ar`, `zip`, `unzip`, `dpkg`, `rpm` |
| `cmd/busybox/` | BusyBox compatibility dispatcher: `busybox` |
| `cmd/debianutils/` | Debian utilities: `run-parts`, `start-stop-daemon`, `pipe_progress` |
| `cmd/fastutil/` | Fast utilities: `fd` (uses `go-fd` and `go-ripgrep` libraries) |
| `cmd/klibc/` | klibc utilities: `nuke`, `resume`, `run-init` |
| `cmd/mailutils/` | Mail utilities: `makemime`, `popmaildir`, `reformime`, `sendmail` |
| `cmd/printutils/` | Print utilities: `lpd`, `lpq`, `lpr` |
| `cmd/selinux/` | SELinux utilities: `chcon`, `getenforce`, `getsebool`, `load_policy`, `matchpathcon`, `restorecon`, `runcon`, `selinuxenabled`, `sestatus`, `setenforce`, `setfiles`, `setsebool` |
| `cmd/networking/` | Network tools: `wget`, `curl`, `ping`, `ping6`, `nc`, `nslookup`, `ifconfig`, `ip` (addr/link/route/rule/neigh), `ipcalc`, `route`, `netstat`, `telnet`, `httpd`, `whois`, `ftpget`, `ftpd`, `tcpsvd`, `brctl`, `ntpd` |
| `cmd/process/` | Process management: `ps`, `kill`, `top`, `uptime`, `free`, `pgrep`, `pkill`, `pmap`, `pidof`, `vmstat`, `fuser`, `iostat`, `lsof`, `pstree`, `pwdx`, `mpstat`, `sysctl`, `killall5` |
| `cmd/shell/` | Shell: `sh`/`ash`/`hush` (interactive + script), `printf`, `shuf` |
| `cmd/editors/` | Editors: `vi`, `ed`, `awk`, `patch`, `sed`, `diff`, `cmp` |
| `cmd/misc/` | Miscellaneous: `less`, `xxd`, `time`, `timeout`, `watch`, `nohup`, `nice`, `killall`, `tty`, `logname`, `awk`, `make`, `bc`, `dc`, `crond`, `crontab`, `devmem`, `hdparm`, `iconv`, `man`, `ts`, `watchdog`, `rfkill`, `adjtimex` |
| `cmd/login/` | Auth: `login`, `su`, `passwd`, `wall`, `last`, `adduser`, `addgroup`, `deluser`, `delgroup`, `mkpasswd`, `vlock`, `sulogin` |
| `cmd/sysklogd/` | Logging: `syslogd`, `klogd`, `logger`, `logread` |
| `cmd/util-linux/` | Linux-specific: `mount`, `umount`, `fdisk`, `blkid`, `lscpu`, `lspci`, `hexdump`, `more`, `dmesg`, `cal`, `eject`, `flock`, `getopt`, `hwclock`, `ionice`, `ipcrm`, `ipcs`, `losetup`, `mountpoint`, `nologin`, `renice`, `script`, `setarch`, `setpriv`, `setsid`, `uuidgen`, `chrt`, `taskset`, `nsenter`, `unshare` |
| `cmd/findutils/` | Find: `find`, `grep`, `egrep`, `fgrep`, `xargs` |
| `cmd/console/` | Console: `clear`, `reset`, `chvt`, `fgconsole`, `openvt`, `setfont`, `resize`, `showkey` |
| `cmd/init/` | Init: `init`, `halt`, `poweroff`, `reboot`, `linuxrc`, `bootchartd` |
| `cmd/runit/` | Runit: `runsv`, `runsvdir`, `sv`, `svc`, `chpst`, `envdir`, `setuidgid`, `softlimit`, `svok`, `svlogd` |
| `cmd/e2fsprogs/` | Ext filesystem: `chattr`, `lsattr`, `tune2fs` |
| `cmd/modutils/` | Kernel modules: `insmod`, `lsmod`, `rmmod`, `modprobe`, `modinfo`, `depmod` |
| `cmd/rootfs/` | Root filesystem generation for containers: `agentbusybox rootfs [dir]` creates a minimal rootfs with all applet symlinks, etc files, and directory structure. Supports `--tar`, `--tar.gz`, `--minimal` formats. |
| `pkg/applet/` | Core applet registry: `Register()`, `Get()`, `Dispatch()`, `List()`. This is the heart of the architecture. |
| `pkg/platform/` | Cross-platform abstractions (OS detection, path handling, shell defaults). |
| `pkg/version/` | Empty directory (reserved) |

## Architecture

### Applet Registry Pattern

Every applet registers itself via `init()` using `applet.Register()`:

```go
func init() {
    applet.Register(&applet.Applet{
        Name:  "ls",
        Short: "List directory contents",
        Func:  runLs,
    })
}
```

The `Applet` struct has fields: `Name`, `Short`, `Func func(args []string) int`, `NoFork bool`, `Usage string`.

`main.go` uses blank imports (`_ "github.com/agentbusybox/cmd/coreutils"`) to trigger `init()` registration. The `applet.Dispatch()` function resolves the applet name from `os.Args[0]` (symlink mode) or `os.Args[1]` (busybox mode) and calls its `Func`.

### Adding a New Applet

1. Choose (or create) a package under `cmd/`.
2. Add a new `.go` file or append to the existing one.
3. Register in `init()` with `applet.Register(...)`.
4. Implement the `Func` — takes `[]string` (full args including applet name), returns exit code.
5. If creating a new package, add a blank import in `main.go`.

### Conventions in Applet Implementation

- Args: `args[0]` is the applet name. User arguments start at `args[1:]`.
- Parse flags manually (no external flag library). Single-char flags are iterated from the flag string (e.g., `-alh`).
- Use `--` to end flag parsing.
- Stderr for errors (`fmt.Fprintf(os.Stderr, ...)`), stdout for output.
- Return 0 for success, 1 for general error, 2 for usage error.
- Stdin fallback: if no files given, read from stdin (often via `"-"` convention).

## Build / Test / Run Commands

```bash
# Build for current platform (output: bin/agentbusybox)
make build

# Run tests
make test          # go test -v -race ./...

# Lint
make lint          # golangci-lint run ./...

# Format code
make fmt           # gofmt -w . && goimports -w .

# Install to $GOPATH/bin
make install

# Build and run
make run

# List registered applets
make applets

# Cross-compile for all platforms
make build-all

# Build distribution packages
make dist

# Clean build artifacts
make clean         # rm -rf bin/
make clean-all     # rm -rf bin/ dist/
```

### Quick one-liners

```bash
go build -o bin/agentbusybox .   # build
go test -v -race ./...           # test
go vet ./...                     # vet (static analysis)
gofmt -l .                       # check formatting
```

## Rootfs Generation

AgentBusyBox can generate a minimal root filesystem for containers:

```bash
# Generate rootfs as a directory
./bin/agentbusybox rootfs ./rootfs

# Generate as tar.gz
./bin/agentbusybox rootfs rootfs.tar.gz --tar.gz

# Generate as tar
./bin/agentbusybox rootfs rootfs.tar --tar

# Minimal (fewer directories)
./bin/agentbusybox rootfs ./rootfs --minimal
```

This creates:
- `/bin/agentbusybox` — the binary
- `/bin/<applet>` — symlinks for all ~389 applets
- `/sbin/<applet>` — symlinks for system commands
- Standard directory structure (`/etc`, `/tmp`, `/var`, `/proc`, `/sys`, `/dev`, `/root`, `/home`)
- Basic config files (`/etc/passwd`, `/etc/group`, `/etc/hostname`, `/etc/hosts`, `/etc/resolv.conf`, `/etc/fstab`, `/etc/inittab`, `/etc/init.d/rcS`)
- Default PATH and PS1 in `/etc/profile`

## Coding Conventions

See `CHECKLIST.md` for the complete flag parity tracking list.

- **Style:** Standard `gofmt`. No external formatter needed.
- **Imports:** Group as: stdlib, then external (`golang.org/x/...`), then internal (`github.com/agentbusybox/...`). `goimports` handles this.
- **Naming:** Follow Go conventions — exported names are PascalCase, unexported are camelCase. Package names are short, lowercase, single-word.
- **Error handling:** Print to stderr and return non-zero exit code. Don't use `log.Fatal` in applets (it calls `os.Exit` which skips deferred cleanup).
- **No external dependencies** beyond `golang.org/x/sys`, `github.com/startvibecoding/go-fd`, and `github.com/startvibecoding/go-ripgrep`. Do not add new dependencies without strong justification.
- **Cross-platform:** Use `pkg/platform` helpers and `runtime.GOOS` checks. Many applets delegate to system commands on non-Linux platforms.
- **Build flags:** The Makefile uses `-trimpath` and ldflags `-s -w` for stripped, reproducible binaries. UPX compression is applied where available.
- **Tests:** Place test files alongside source (`*_test.go`). Use `go test -v -race ./...`.

## Agent Rules

### 核心原则：纯 Go 实现，禁止 exec.Command

**本项目的目标是创建一个完全独立的 Go 二进制文件，不依赖任何系统命令。**

- **绝对禁止** 在 `cmd/` 目录下的 applet 实现中使用 `exec.Command` 来调用外部系统命令（如 `ping`, `mount`, `traceroute`, `bzip2`, `ar`, `rpm`, `dpkg`, `chattr`, `lsmod`, `rfkill`, `hdparm`, `i2c*` 等）。
- **唯一例外**：`cmd/shell/` 包中的 shell 命令（`sh`, `ash`, `hush`）可以执行外部命令，因为 shell 的本质就是执行外部程序。此外，`nohup`, `nice`, `timeout`, `watch`, `killall`, `make` 等命令本身就是为了执行其他程序而设计的，这些可以使用 `exec.Command`。
- **所有其他 applet** 必须使用 Go 标准库（`net`, `os`, `syscall`, `unsafe`, `compress/*`, `crypto/*` 等）或读取 `/proc`, `/sys`, `/dev` 来实现功能。
- 如果某个功能在纯 Go 中暂时无法实现（如 ioctl 调用），应返回“not yet implemented in pure Go”错误，而不是委托给系统命令。
- **正确的实现方式**：使用 `net.Dial("ip4:icmp", ...)` 实现 ping，使用 `os.ReadFile("/proc/modules")` 实现 lsmod，使用 `os.OpenFile("/dev/console", ...)` 实现 console 操作，使用 Go 的 `compress/gzip`, `archive/tar`, `archive/zip` 等实现压缩解压。

### 其他规则

- **Do not add new module dependencies** without explicit user approval.
- **Do not delete or rewrite existing applet implementations** unless specifically asked. Prefer editing in place.
- **Do not change the applet registry pattern.** All new applets must use `applet.Register()` in `init()`.
- **Do not remove blank imports in `main.go`** for existing cmd packages.
- **Keep all files under `cmd/` and `pkg/`** — no new top-level packages.
- **Preserve the Makefile** build targets and variables. Do not switch to a different build system.
- **Run `make fmt` before committing** to ensure formatting consistency.
- **Run `make test` after changes** to verify nothing breaks.
- **Binary file** `agentbusybox` at the root is a prebuilt binary — do not modify or delete it.
