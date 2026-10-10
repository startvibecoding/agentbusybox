# AGENTS.md — busybox-w32 项目指南(中文)

## 项目简介

本项目是 **BusyBox for Windows**(busybox-w32),上游为 <https://github.com/rmyorston/busybox-w32>,
基于上游 busybox 1.38 的 fork(本 fork 版本号自 v1.38.1 起),核心目标是在 Microsoft Windows 上提供 BusyBox 功能。

**本项目定位:提供给 AI agent 使用**(运行环境为 Windows)。因此已**精简掉 Linux 独占能力**:
移除全部 Linux 内核/系统级 applet(内核模块、init/启动、SELinux、挂载/分区、网络配置、
/proc 进程工具、控制台硬件、登录/用户管理、守护进程框架等),源码树大幅瘦身。
Linux 构建保留,作为开发与验证环境(用 `make defconfig` 构建交叉平台的 applet 子集)。

关键设计:一套代码、两种目标平台,由 kconfig 选项 `PLATFORM_POSIX` / `PLATFORM_MINGW32` 切换:

- POSIX 平台 = 跨平台 applet 子集在 Linux 上的构建
- MINGW32 平台 = 通过 `win32/` 目录的 POSIX 兼容层 + `#if ENABLE_PLATFORM_MINGW32` 条件代码,
  让同一套 applet 源码在 Windows 上编译运行(**这是本 fork 的核心价值**)

## 快速构建(在 Linux 上交叉编译)

源码树必须先 `make mrproper` 干净后才能用 `O=` 外置构建(见"常见陷阱")。仓库根目录的
`build.sh` 一键构建:

```sh
./build.sh          # 同时构建 Linux 版和 Windows x86_64 版
./build.sh linux    # 只构建 Linux 版
./build.sh win64    # 只构建 Windows 版
```

手动流程:

```sh
mkdir -p build-linux && make O=build-linux defconfig && make O=build-linux -j$(nproc)
mkdir -p build-win64 && make O=build-win64 mingw64_defconfig && make O=build-win64 -j$(nproc)
```

产物:Linux 为 `build-linux/busybox`(~590KB、194 applet),Windows 为 `build-win64/busybox.exe`
(~712KB、179 applet)。Windows 产物可用 `WINEDEBUG=-all wine build-win64/busybox.exe --list` 冒烟测试。

可用 defconfig(`configs/` 目录,已精简为 Windows 相关配置):`mingw32_defconfig`(32 位)、
`mingw32w_defconfig`(32 位 UTF-8)、`mingw64_defconfig`(64 位)、`mingw64u_defconfig`(64 位 UTF-8)、
`mingw64a_defconfig`,以及通用 `make defconfig`(Linux/POSIX)。
`scripts/mk_mingw64u_defconfig` 由 mingw64 配置派生 UTF-8 配置。

本机环境:gcc 14、`x86_64-w64-mingw32-gcc` / `i686-w64-mingw32-gcc` 均已安装,wine 可用于测试。

## 构建系统(已瘦身)

顶层只保留六个核心动作:配置(`*config`/`menuconfig`)、构建(`make [O=dir]`)、
清理(`clean`/`mrproper`/`distclean`)、测试(`check`/`test`)、安装(`install`)、帮助(`help`)。
构建引擎仍是 Linux 内核 kbuild 的递归 make(内核遗留的外部模块、modules、kallsyms、
打包、编辑器索引等死代码已删除),关键文件:

- `Makefile` — 顶层调度(O= 二次 make 重定向、*config 分流、递归下降、链接与 strip)
- `Makefile.flags` — 按 .config 拼装 CFLAGS/LDFLAGS/LDLIBS(含 MINGW32 分支)
- `Makefile.custom` — install/check/doc/尺寸分析类目标
- `Makefile.help` — help 文本
- `scripts/Kbuild.include`、`scripts/Makefile.build`、`scripts/Makefile.lib`、
  `scripts/Makefile.host`、`scripts/Makefile.clean` — kbuild 引擎(未改动,勿随意重构)
- `scripts/gen_build_files.sh` — applet 元数据机制的实现(见"配置项定义")

重要约定:`scripts/Makefile.build` 用 `$(if $(KBUILD_BUILTIN),...)` 决定每个目录产出什么,
顶层 Makefile 必须 `export KBUILD_BUILTIN=1`(以及 `KBUILD_CHECKSRC:=0`,否则每个 .o
会被当成 sparse 检查目标而编译失败)。`all:` 不再依赖 `doc`,文档需显式 `make doc`。
`make test` 会把 `testsuite/` 和 `scripts/echo.c` 复制到构建目录(dash 等不自带可用的
`echo -ne`,测试脚本按 `../scripts/echo.c` 相对路径编译辅助 echo)。

## 目录结构

标准 busybox 目录(coreutils/、shell/、networking/、archival/、editors/ 等按功能分组的 applet 源码)
之外,本 fork 特有的部分:

- `win32/` — **Windows 平台层**,整个 fork 的核心。包含:
  - `mingw.c`(~2700 行):POSIX 仿真主文件(stat/open/readdir/fork 语义等)
  - `process.c`:进程与子进程管理(用 Windows 线程仿真 fork/waitpid)
  - `winansi.c`:ANSI 转义序列与 Windows 控制台 API 之间的仿真;
    其中控制台输出经 `conv_fwriteCon`/`conv_writeCon` 做编码处理:
    `FEATURE_UTF8_OUTPUT` 假定输出全部是 UTF-8;`FEATURE_UTF8_AUTODETECT`
    (本 fork 新增)则以"非 ASCII 字节段"为单位自动探测:整段严格合法 UTF-8
    则按 UTF-8 解码;否则在段内交替——把"含 3/4 字节序列的合法 UTF-8 连续段"
    判为 UTF-8,其余按传统代码页解码(默认取控制台输出 CP,非 UTF-8 时;
    否则用进程 ACP;可用 `BB_CONS_ENCODING=<cp>` 覆盖,如 936),
    统一经 `WriteConsoleW` 写出,解决 UTF-8/GBK 等混合输出的中文乱码
  - `poll.c` / `select.c`:套接字 I/O 复用仿真;`popen.c`、`termios.c`、`mntent.c`、`glob.c`、
    `fnmatch.c`、`fnmatch2.c`、`regex*.c`(从 glibc 移植的正则)、`strptime.c` 等:libc 缺失函数补齐
  - `*.h` 影子头文件(dirent.h、glob.h、regex.h、poll.h、termios.h、mntent.h 等)
    以及 `sys/`、`net/`、`arpa/`、`netinet/` 下的系统头文件影子
  - `resources/`:**Windows 专属资源**(ICO/PNG 图标、VERSIONINFO 版本信息、manifest),
    由 `Kbuild.src` 里的 `resources.rc` 生成,对应 Config.in 里的 `FEATURE_RESOURCES` 系列选项
  - `Kbuild.src`:平台层自身的编译清单,所有 .o 都挂 `lib-$(CONFIG_PLATFORM_MINGW32)`
- `include/mingw.h` — 大量内联兼容垫片(类型别名、宏、NOIMPL 桩函数),通过 `-Iwin32` 与
  `include/platform.h` / `include/libbb.h` 接入
- `loginutils/suw32.c` — Windows 专属的 `su` applet(通过 ShellExecute 提权),"ODDNAME 命名"典型例子
- `configs/mingw*_defconfig` — Windows 各变体默认配置
- `scripts/mk_mingw64u_defconfig` — 派生 UTF-8 配置的脚本

## Windows 平台层如何接入(改动前的必读)

平台层有四个接入点,动任何一个都要保证两边都能编过:

1. **编译标志**:`Makefile.flags` 中 `ifeq ($(CONFIG_PLATFORM_MINGW32),y)` 分支 —
   `-Iwin32`(让影子头文件遮蔽系统头)、`-DHAVE_STRING_H=1` 等、`EXEEXT = .exe`、
   `LDLIBS += ws2_32 bcrypt secur32`。非 MINGW 分支则 `LDLIBS += m crypt rt`(按可用性探测)
2. **平台检测**:`include/platform.h` — 定义 `ENABLE_PLATFORM_MINGW32`、`_WIN32_WINNT 0x502`;
   若选了 POSIX 却用 MinGW 编译器编译会直接 `#error`
3. **兼容垫片**:`include/libbb.h` 中 `#if ENABLE_PLATFORM_MINGW32 # include "mingw.h" #endif`,
   `MINGW_SPECIAL(a)` 宏把 `stat` 等替换为 `mingw_stat`(win32/mingw.c 里的实现)
4. **条件代码**:若干 .c/.h 文件里散落 `#if ENABLE_PLATFORM_MINGW32`(或 `#ifndef` 变体)
   处理路径分隔符、大小写不敏感文件系统、无 fork 环境等差异。新增平台相关分支时优先复用
   `win32/mingw.c` 与 `include/mingw.h`,避免到处堆 `#ifdef`

配置项定义:applet 的配置在 `.c` 文件头部 `//config:` 注释块里,各目录 `Config.src` 只放共享
FEATURE 选项和菜单结构;`scripts/gen_build_files.sh` 据此生成 `*/Config.in`、`*/Kbuild`
(生成物不提交,只有根 `Config.in` 历史遗留被跟踪)。

## 新增/修改 applet 的约定(沿用 busybox 规则)

新增 applet 只需写一个 `<applet>.c` 文件,并在文件头部用特殊注释块声明元数据
(构建系统自动生成 applet 表、usage、配置),格式见 `docs/new-applet-HOWTO.txt`:

```c
//config:config MU
//config:	bool "MU"
//config:	default y
//config:	help
//config:	  ...
//kbuild:lib-$(CONFIG_MU) += mu.o
//applet:IF_MU(APPLET(mu, BB_DIR_USR_BIN, BB_SUID_DROP))
//usage:#define mu_trivial_usage ...
int mu_main(int argc, char **argv) MAIN_EXTERNALLY_VISIBLE;
```

要点:
- `#include "libbb.h"` 必须是第一个 include(它已包含常用系统头)
- 入口函数命名 `<applet>_main` 且加 `MAIN_EXTERNALLY_VISIBLE`
- 多命令名用 `APPLET_ODDNAME`(参考 `loginutils/suw32.c`)
- 共享库文件(无 //applet 的 .c)通过 `//kbuild:lib-$(CONFIG_XXX) += xxx.o` 挂在需要它的
  applet 上;删除 applet 时**必须检查**其 kbuild 引用的共享 .o 是否还有别的引用者,
  否则会变成无人引用的死代码(例如 `libbb/ioloop.c` 原挂在 TELNETD 上,现改为挂在 TELNET)
- 代码风格:文件头 `/* vi: set sw=4 ts=4: */`,4 空格缩进,tab 用于行首对齐

## 常见陷阱(踩坑记录)

1. **O= 外置构建要求源码树干净**:若源码根目录存在 `.config` 或 `include/BB_VER.h` 等生成物,
   `make O=...` 会报 "source is not clean"。必须先 `make mrproper`(git 树干净时只删生成文件)
2. **别在源码根目录直接 make**:会污染源码树,破坏后续 O= 构建;一律用 `O=build-*`
3. **构建目录勿提交**:`build-linux/`、`build-win64/` 是本地产物;`build.sh` 是辅助脚本
4. **平台头文件遮蔽**:改 `win32/*.h` 影子头文件会影响所有 `-Iwin32` 编译单元,改动前先确认
   对应的 Windows 原生头文件行为
5. **删除 applet 的连锁反应**:除了 .c 文件,还要检查——静态 `Kbuild.src` 里引用其配置符号的
   行、`Makefile` 的 `libs-y` 目录表、`Config.src` 里的
   `source` 子目录行、`include/libbb.h` 等公共头里的 `ENABLE_XXX` 引用、共享库的 kbuild 挂载、
   testsuite/ 与 NOFORK_NOEXEC.lst、configs/*_defconfig(用 `make O=/tmp/x <name>_defconfig`
   重新生成即可归一化掉死符号)
6. **kconfig 语法**:`//config:` 注释块生成 Config.in,注释行必须以 `//config:#` 开头才是合法
   kconfig 注释,裸文本会导致 "unknown option" 错误
7. **条件编译块编辑**:`include/libbb.h` 等公共头里删除 `#if` 块时,注意嵌套配对
   (#if/#ifdef/#ifndef/#elif/#else/#endif 全部参与),建议用完整检查器验证平衡

## 测试

- Linux 产物:直接运行 `build-linux/busybox <applet> ...`;全量回归用 `make O=build-linux test`
  (testsuite/,含 `all_sourcecode.tests` 源码完整性检查)
- Windows 产物:本机用 wine 冒烟:`WINEDEBUG=-all wine build-win64/busybox.exe --list`
  (wine 已安装);`--list` 可统计 applet 数,快速判断配置是否生效
- 修改平台层后建议两个平台都构建一次,确认没有破坏另一边

## 已移除内容(精简记录)

为聚焦 Windows agent 场景移除的 Linux 独占子系统(整目录):

- `modutils/`(内核模块)、`init/`(启动/关机)、`selinux/`、`sysklogd/`、`klibc-utils/`(引导工具)
- `runit/`(守护进程监督)、`mailutils/`、`printutils/`(打印守护进程)
- `networking/udhcp/`(DHCP)、`networking/libiproute/`(ip 命令套件)
- `util-linux/volume_id/`(卷标识)、`qemu_multiarch_testing/`(QEMU 测试基础设施)

以及分散的 Linux 内核级 applet:内核控制台工具、/proc 系进程工具(top/sysctl/lsof/fuser 等)、
挂载/分区/交换( mount/umount/fdisk/mkswap 等)、网络配置(ifconfig/ip/route/arp/brctl/tc/ping 等)、
登录/用户管理(passwd/login/adduser 等)、Linux 杂项(devmem/hdparm/i2c/MTD/UBI 等)。
移除后公共头文件(libbb.h)、共享库 kbuild 挂载、Makefile 目录表均已同步清理。

构建系统也已精简(顶层 `Makefile` 1417→693 行,详见上节):删掉从未可用的 Linux 内核构建遗留——
kallsyms/System.map 三段链接、可加载模块与外部模块(`make modules`、`M=`、MODVERDIR、
depmod/modpost)、sparse 静态检查(`make C=1`)、`%pkg`/rpm 打包、DocBook `%docs`、
includecheck/versioncheck/namespacecheck(对应 perl 脚本已不存在)、include2 asm 符号链接、
`INSTALL_PATH=/boot`、KBUILD_IMAGE、UTS_MACHINE、version.h filechk、FLTFLAGS(bFLT)、
`.kernelrelease` 机制、`config` 与构建目标的混合调用、编辑器索引(tags/cscope)及其脚本、
`checkstack`/`kernelrelease`/`kernelversion` 目标、`.lst` 目标(scripts/makelst 已不存在)、
HOSTCXX(`scripts/kconfig` 的 GUI 配置器已移除,不再需要 C++ 宿主程序);`scripts/Makefile.build`
同步清理(checksrc、MODVERSIONS、single/multi-used-m 规则)。保留全部真实功能:`O=` 外置构建、
arch/*/Makefile 的 CFLAGS 调整、`*config` 配置、install、clean/mrproper/distclean、
check/test、help,以及 `Makefile.custom` 中的尺寸分析目标(sizes/bloatcheck/stksizes 等)。
验证:Linux 产物与精简前逐字节一致(仅时间戳不同),Windows mingw 交叉构建产物 179 applet 不变。

如需恢复某个 applet,`git log` 或上游 busybox 可找回对应 .c 文件,补上即可。
