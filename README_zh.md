# busybox-w32 — 为 AI Agent 精简的 BusyBox for Windows

本项目基于 [busybox-w32](https://github.com/rmyorston/busybox-w32)(BusyBox 1.38 的 Windows 移植版),
定位为**提供给 AI Agent 的命令行工具环境**,已精简掉 Linux 独占能力
(内核模块、init/启动、SELinux、挂载/分区、网络配置、/proc 进程工具、登录管理、守护进程框架等),
保留**跨平台通用工具集** + **Windows 平台层**(`win32/` + `include/mingw.h`)。

一套源码,两种目标平台:

- **Windows**(主要目标,agent 运行环境):`make mingw64_defconfig` 交叉编译出 `busybox.exe`
- **Linux/POSIX**(开发与验证环境):`make defconfig` 编译跨平台子集

---

## 快速构建(在 Linux 上交叉编译)

```sh
./build.sh          # 同时构建 Linux 版和 Windows x86_64 版
./build.sh linux    # 只构建 Linux 版
./build.sh win64    # 只构建 Windows 版
```

产物:`build-linux/busybox`(ELF,约 590KB、194 个工具)、`build-win64/busybox.exe`(PE32+,约 712KB、179 个工具)。
Windows 版可用 wine 冒烟测试:`WINEDEBUG=-all wine build-win64/busybox.exe --list`。

手动流程:

```sh
make mrproper                    # O= 外置构建要求源码树干净(仅此一次或每次改动源码树后)
mkdir -p build-linux && make O=build-linux defconfig && make O=build-linux -j$(nproc)
mkdir -p build-win64 && make O=build-win64 mingw64_defconfig && make O=build-win64 -j$(nproc)
```

其他可用配置:`configs/` 下的 `mingw32_defconfig`(32 位)、`mingw32w_defconfig`(32 位 UTF-8)、
`mingw64u_defconfig`(64 位 UTF-8)、`mingw64a_defconfig`。

## 使用方法

单二进制多命令:

```sh
busybox.exe ls -la          # 直接指定工具名调用
busybox.exe --list          # 列出内置的全部工具
cp a b                      # 或用符号链接/副本命名为工具名,直接调用
```

内置 ash 兼容 shell(`sh`/`bash` 别名)支持管道、重定向、通配符、作业控制;
shell 内可直接调用内置工具(standalone 模式),无需安装到 PATH。

Windows 路径说明:统一使用正斜杠 `c:/path`、`//host/share/path`;
用户/组/权限为仿真值;提供 `/dev/null`、`/dev/tty`、`/dev/zero`、`/dev/urandom` 等设备文件仿真。

---

## 工具清单(按功能分组)

> 括号内为同名别名。标 ★ 的为 Windows 专属或仅在 Windows 版中启用。

### Shell 与命令环境

| 工具 | 用途 |
|---|---|
| `ash` (`sh`/`bash`/`lash`) | Bourne 兼容 shell,支持管道、重定向、作业控制、历史记录、命令行编辑 |
| `hush` (`sh`) | 更小的 shell 实现,资源占用更低,可替代 ash |
| `busybox` | 多命令调度本体,`busybox <工具> <参数>` 或 `busybox --list` |

### 文件与目录操作

| 工具 | 用途 |
|---|---|
| `ls` (`dir`) | 列出目录内容,支持颜色、排序、时间、递归 |
| `cp` | 复制文件/目录(支持 `-r`、`-a`、`-p`、`-i`、`-l`、`-s`、`-u`) |
| `mv` | 移动/重命名 |
| `rm` | 删除文件/目录(`-r` 递归、`-f` 强制、`-i` 交互) |
| `rmdir` | 删除空目录 |
| `mkdir` | 创建目录(`-p` 递归创建) |
| `touch` | 更新文件时间戳或创建空文件 |
| `ln` / `link` | 创建硬链接/符号链接 |
| `unlink` | 删除单个文件 |
| `install` | 复制文件并设置权限/属主,常用于安装脚本 |
| `chmod` | 修改权限 |
| `chown` / `chgrp` | 修改属主/属组(Windows 上为仿真语义) |
| `chroot` | 切换根目录后执行命令(仅 Linux) |
| `mkfifo` / `mknod` | 创建命名管道 / 设备节点(仅 Linux) |
| `nice` / `nohup` | 以调整优先级运行 / 忽略挂起信号运行 |
| `pwd` | 显示当前工作目录 |
| `chattr` / `lsattr` | 修改/查看文件扩展属性(Windows 上支持只读、系统、隐藏等标志) |
| `stat` | 查看文件详细状态(大小、权限、时间戳、inode 等) |
| `readlink` / `realpath` | 解析符号链接目标 / 输出规范绝对路径 |
| `basename` / `dirname` | 取路径的文件名部分 / 目录部分 |
| `mktemp` | 创建临时文件/目录并输出其路径 |
| `truncate` | 将文件截断/扩展为指定大小 |
| `dd` | 块级复制与转换(指定输入输出偏移、块大小) |
| `shred` | 覆写文件内容以安全删除 |
| `fsync` / `sync` | 将文件 / 全部缓冲写入磁盘 |
| `flock` | 对文件加锁执行命令 |

### 文本查看与处理

| 工具 | 用途 |
|---|---|
| `cat` | 连接并打印文件 |
| `tac` | 逆序打印文件行 |
| `tee` | 同时输出到标准输出和文件 |
| `head` / `tail` | 查看文件开头 / 结尾(`-f` 跟踪追加) |
| `wc` | 统计行数、词数、字节数 |
| `cut` | 按字段/字节/字符切取列 |
| `tr` | 字符集替换/删除 |
| `sort` | 排序(支持 `-n` 数值、`-r` 逆序、`-k` 键位、`-u` 去重) |
| `uniq` | 去除相邻重复行 |
| `comm` | 两文件逐行比较(求交集/差集) |
| `join` | 按公共字段连接两个文件 |
| `paste` | 按行合并多个文件 |
| `nl` | 给行加编号 |
| `fold` / `expand` / `unexpand` | 折行 / tab 与空格互转 |
| `od` / `hexdump` / `hd` / `xxd` | 十六进制转储(二进制分析) |
| `strings` | 提取二进制文件中的可打印字符串 |
| `split` | 按大小/行数拆分文件 |
| `sed` | 流编辑器:查找替换、行过滤、脚本化文本变换 |
| `awk` | 文本处理语言:按字段解析、统计、生成报告 |
| `find` | 按名称/类型/时间/大小等条件查找文件 |
| `xargs` | 从标准输入读取参数并构建命令执行 |
| `grep` (`egrep`/`fgrep`) | 正则/扩展正则/固定串搜索 |
| `diff` / `cmp` | 逐行 / 逐字节比较文件 |
| `patch` | 应用 diff 补丁 |
| `ed` | 经典行编辑器 |
| `vi` | 全屏编辑器(支持搜索、替换、撤销、多文件) |
| `less` / `more` | 分页查看(less 支持向上翻页、搜索) |

### 编码、校验与转换

| 工具 | 用途 |
|---|---|
| `base32` / `base64` | Base32/Base64 编解码 |
| `uuencode` / `uudecode` | UU 编解码 |
| `md5sum` / `sha1sum` / `sha256sum` / `sha384sum` / `sha512sum` / `sha3sum` | 各类哈希校验 |
| `cksum` / `crc32` / `sum` | 校验和 / CRC32 / BSD sum |
| `dos2unix` / `unix2dos` | 换行符转换 |
| `iconv` | 字符集编码转换 |

### 归档与压缩

| 工具 | 用途 |
|---|---|
| `tar` | 打包/解包(支持创建、追加、自动识别压缩格式) |
| `cpio` | cpio 归档(常用于 initramfs 等) |
| `ar` | 静态库/归档维护 |
| `gzip`/`gunzip` (`zcat`) | gzip 压缩/解压 |
| `bzip2`/`bunzip2` (`bzcat`) | bzip2 压缩/解压 |
| `xz`/`unxz` (`xzcat`) | xz 压缩/解压 |
| `lzma`/`unlzma` (`lzcat`)、`lzop`/`unlzop` (`lzopcat`) | 其他 LZ 系压缩 |
| `uncompress` | 旧式 .Z 解压 |
| `unzip` | ZIP 解压(支持常见压缩算法) |
| `dpkg` / `dpkg-deb` | Debian 包(.deb)查询/解包 |
| `rpm` / `rpm2cpio` | RPM 包查询 / 转为 cpio 流 |

### 进程与系统

| 工具 | 用途 |
|---|---|
| `ps` (`minips`) | 查看进程列表(Windows 上基于任务快照) |
| `kill` | 按 PID 终止进程 |
| `killall` | 按名称终止进程 |
| `killall5` | 终止除自身外的全部进程(仅 Linux) |
| `pgrep` / `pkill` | 按名称/属性匹配进程:查找 / 终止 |
| `pidof` | 按名称查 PID |
| `uptime` | 系统运行时间与负载 |
| `free` | 内存使用情况(Windows 上基于系统信息) |
| `watch` | 周期性重复执行命令并刷新输出 |
| `time` | 测量命令执行耗时 |
| `nproc` | CPU 逻辑核心数 |
| `uname` (`arch`) | 系统/内核信息 |
| `hostname` / `dnsdomainname` | 主机名 / DNS 域名 |
| `id` / `groups` / `whoami` / `logname` | 当前用户身份信息(Windows 上为仿真) |
| `tty` / `ttysize` | 终端设备名 / 终端行列数 |
| `date` / `cal` | 日期时间(支持格式化、设置)/ 日历 |
| `rdate` | 从远程时间服务器获取时间 |
| `df` / `du` | 磁盘空间使用 / 目录占用空间 |
| `who` / `users` / `w` | 登录用户列表 / 用户会话与负载信息 |
| `hostid` | 主机 ID(仅 Linux) |

### 网络

| 工具 | 用途 |
|---|---|
| `wget` | HTTP/HTTPS/FTP 下载(支持断点续传、认证、HTTPS) |
| `nc` (`netcat`) | TCP/UDP 连接工具:端口测试、简易服务端/客户端、数据传输 |
| `telnet` | 交互式 Telnet 客户端 |
| `tftp` / `tftpd` | TFTP 客户端 / 简易 TFTP 服务端 |
| `ftpget` / `ftpput` | FTP 下载 / 上传(脚本化,非交互) |
| `httpd` ★ | 内嵌 HTTP 服务端(静态文件、CGI、认证、gzip) |
| `nslookup` | DNS 查询 |
| `whois` | WHOIS 域名/IP 查询 |
| `ipcalc` | IP/子网计算(掩码、网络地址、广播地址) |
| `pscan` | 简易端口扫描器 |
| `ssl_client` | 建立 TLS/SSL 连接(配合 wget 等的内部 TLS 实现) |

### 数学与逻辑

| 工具 | 用途 |
|---|---|
| `bc` / `dc` | 任意精度计算器(交互/脚本)/ 逆波兰计算器 |
| `expr` | 表达式求值(算术、字符串、比较) |
| `factor` | 质因数分解 |
| `seq` | 生成等差数列 |
| `printf` | 格式化输出 |
| `echo` | 输出文本(`-e` 转义、`-n` 不换行) |
| `test` (`[`) | 条件测试(文件、字符串、数值) |
| `true` / `false` | 恒真 / 恒假命令 |
| `yes` | 无限重复输出字符串 |
| `sleep` / `usleep` | 睡眠指定秒数 / 微秒数 |
| `timeout` | 限时执行命令,超时终止 |
| `shuf` | 随机打乱输入行 |
| `tsort` | 拓扑排序 |

### 环境与 shell 辅助

| 工具 | 用途 |
|---|---|
| `env` | 显示/设置环境变量后执行命令 |
| `printenv` | 打印环境变量 |
| `export`/`set`/`unset`/`cd` 等 | shell 内建(见 ash) |
| `getopt` | 解析命令行选项(供脚本使用) |
| `which` | 定位可执行文件路径 |
| `run-parts` | 按序执行目录下的所有脚本 |
| `pipe_progress` | 管道进度显示(供其他工具内部使用) |
| `make` (`pdpmake`) | 内嵌的 make 构建工具(POSIX 兼容) |
| `man` | 查看手册页 |

### 开发与调试辅助

| 工具 | 用途 |
|---|---|
| `xxd`/`hexdump`/`od` | 十六进制转储(见文本处理) |
| `hexedit` | 交互式十六进制编辑器 |
| `strings` | 提取二进制字符串 |
| `stty` | 设置终端参数 |
| `clear` / `reset` | 清屏 / 重置终端 |
| `resize` | 输出调整终端尺寸的 shell 命令 |
| `script` / `scriptreplay` | 录制终端会话 / 回放(便于复现与审计) |
| `uuidgen` | 生成 UUID |
| `rev` | 反转每行字符顺序 |
| `tree` | 以树状显示目录结构 |
| `ascii` ★ | 显示 ASCII 字符表 |
| `bbconfig` | 输出当前构建配置 |
| `jn` | JSON 数据处理工具 |
| `ts` | 时间戳工具 |
| `inotifyd` | 文件系统事件监控(基于 Windows 通知) |
| `crond` / `crontab` | 定时任务守护 / 任务表管理 |
| `chat` | 串口脚本对话(供拨号等使用) |
| `microcom` | 简易串口终端 |
| `rx` | XMODEM 接收 |

### Windows 专属

| 工具 | 用途 |
|---|---|
| `su` (suw32) ★ | 以管理员权限运行 shell 或命令(ShellExecute 提权,支持 `-c` 命令、`-t` 测试模式) |
| `drop` / `cdrop` / `pdrop` ★ | 降低进程权限令牌,以普通权限运行子进程 |
| `chattr` / `lsattr` | Windows 文件标志(只读/系统/隐藏)管理 |

### 测试与内部

| 工具 | 用途 |
|---|---|
| `unit` | 单元测试框架入口(默认关闭) |
| `parse` | 配置解析调试工具(默认关闭) |

---

## 设计要点与提示

- **standalone shell**:shell 内直接调用内置工具,不依赖 PATH;可用环境变量
  `BB_OVERRIDE_APPLETS`(空格分隔的工具名列表)强制改用外部程序
- **终端模式**:自动检测最佳终端处理方式;`BB_TERMINAL_MODE=1` 强制 ANSI 直通,
  `BB_TERMINAL_MODE=0` 强制用 Windows 控制台 API 仿真
- **设备文件仿真**:`/dev/null`、`/dev/tty`、`/dev/zero`、`/dev/urandom` 可用于重定向与参数
- **路径**:一律用正斜杠;网络路径 `//host/share/path`;`-X` shell 选项可禁止把反斜杠转为正斜杠
- **安全提示**:把 busybox 进程加入 Windows 安全中心的排除项可显著提升性能
- **系统要求**:Windows 7+ / Server 2008 R2+(UTF-8 变体需 Windows 10+)

## 更多信息

- 完整工具与选项:`busybox.exe --list`、`busybox.exe <工具> --help`
- 开发与维护说明见 [AGENTS.md](AGENTS.md)(含平台层架构、新增/删除 applet 的约定与连锁反应检查清单)
