# busybox-w32 — BusyBox for Windows, trimmed for AI agents

This project is based on [busybox-w32](https://github.com/rmyorston/busybox-w32) (the Windows port of
BusyBox 1.38). It is positioned as a **command-line tool environment for AI agents**, and has been
trimmed of Linux-exclusive capabilities (kernel modules, init/boot, SELinux, mount/partitioning,
network configuration, /proc-based process tools, login/user management, daemon frameworks, etc.),
keeping the **cross-platform toolset** plus the **Windows platform layer** (`win32/` + `include/mingw.h`).

One source tree, two target platforms:

- **Windows** (primary target, the agent runtime): `make mingw64_defconfig` cross-compiles `busybox.exe`
- **Linux/POSIX** (development and verification environment): `make defconfig` builds the cross-platform subset

---

## Building (cross-compile on Linux)

```sh
./build.sh          # build both Linux and Windows x86_64 versions
./build.sh linux    # Linux only
./build.sh win64    # Windows only
```

Outputs: `build-linux/busybox` (ELF, ~590 KB, 194 applets), `build-win64/busybox.exe`
(PE32+, ~712 KB, 179 applets). Smoke-test the Windows build with wine:
`WINEDEBUG=-all wine build-win64/busybox.exe --list`.

Manual steps:

```sh
make mrproper                    # O= out-of-tree builds require a clean source tree
mkdir -p build-linux && make O=build-linux defconfig && make O=build-linux -j$(nproc)
mkdir -p build-win64 && make O=build-win64 mingw64_defconfig && make O=build-win64 -j$(nproc)
```

Other configurations in `configs/`: `mingw32_defconfig` (32-bit), `mingw32w_defconfig` (32-bit UTF-8),
`mingw64u_defconfig` (64-bit UTF-8), `mingw64a_defconfig`.

## Usage

One binary, many commands:

```sh
busybox.exe ls -la          # invoke an applet directly
busybox.exe --list          # list all built-in applets
cp a b                      # or name a link/copy after the applet and call it directly
```

The built-in ash-compatible shell (aliased as `sh`/`bash`) supports pipes, redirection, wildcards and
job control; built-in applets are called directly from the shell (standalone mode), no PATH install needed.

Windows path notes: always use forward slashes (`c:/path`, `//host/share/path`); users, groups and
permissions are emulated; device-file emulation provides `/dev/null`, `/dev/tty`, `/dev/zero`,
`/dev/urandom`.

---

## Applet reference (grouped by function)

> Names in parentheses are aliases. ★ marks Windows-specific applets or features enabled only in the
> Windows build.

### Shell & command environment

| Applet | Purpose |
|---|---|
| `ash` (`sh`/`bash`/`lash`) | Bourne-compatible shell: pipes, redirection, job control, history, line editing |
| `hush` (`sh`) | Smaller shell implementation with lower resource use; drop-in alternative to ash |
| `busybox` | Multi-call dispatcher: `busybox <applet> <args>` or `busybox --list` |

### Files & directories

| Applet | Purpose |
|---|---|
| `ls` (`dir`) | List directory contents (color, sorting, timestamps, recursion) |
| `cp` | Copy files/directories (`-r`, `-a`, `-p`, `-i`, `-l`, `-s`, `-u`) |
| `mv` | Move/rename |
| `rm` | Remove files/directories (`-r` recursive, `-f` force, `-i` interactive) |
| `rmdir` | Remove empty directories |
| `mkdir` | Create directories (`-p` parents) |
| `touch` | Update timestamps or create empty files |
| `ln` / `link` | Create hard/symbolic links |
| `unlink` | Remove a single file |
| `install` | Copy files with permissions/owner set (used by install scripts) |
| `chmod` | Change permissions |
| `chown` / `chgrp` | Change owner/group (emulated on Windows) |
| `chattr` / `lsattr` | Change/show extended attributes (read-only, system, hidden flags on Windows) |
| `stat` | Detailed file status (size, permissions, timestamps, inode, ...) |
| `readlink` / `realpath` | Resolve symlink target / print canonical absolute path |
| `basename` / `dirname` | Strip directory part / strip filename part of a path |
| `mktemp` | Create a temporary file/dir and print its path |
| `truncate` | Shrink/extend a file to a given size |
| `dd` | Block-level copy and conversion (offset, block size, ...) |
| `shred` | Overwrite file contents for secure deletion |
| `fsync` / `sync` | Flush file / all buffers to disk |
| `flock` | Run a command while holding a file lock |
| `chroot` | Run a command with a different root directory (Linux only) |
| `mkfifo` / `mknod` | Create named pipes / device nodes (Linux only) |
| `nice` / `nohup` | Run with adjusted priority / immune to hangup |
| `pwd` | Print current working directory |

### Text viewing & processing

| Applet | Purpose |
|---|---|
| `cat` | Concatenate and print files |
| `tac` | Print lines in reverse order |
| `tee` | Write to stdout and files simultaneously |
| `head` / `tail` | First / last lines of a file (`-f` follow) |
| `wc` | Count lines, words, bytes |
| `cut` | Extract columns by field/byte/char |
| `tr` | Character translation/deletion |
| `sort` | Sort lines (`-n` numeric, `-r` reverse, `-k` key, `-u` unique) |
| `uniq` | Drop adjacent duplicate lines |
| `comm` | Compare two sorted files line by line |
| `join` | Join two files on a common field |
| `paste` | Merge lines of multiple files |
| `nl` | Number lines |
| `fold` / `expand` / `unexpand` | Wrap lines / convert tabs and spaces |
| `od` / `hexdump` / `hd` / `xxd` | Hex dumps (binary analysis) |
| `strings` | Extract printable strings from binaries |
| `split` | Split files by size/lines |
| `find` | Find files by name/type/time/size conditions |
| `xargs` | Build and run commands from stdin arguments |
| `sed` | Stream editor: search/replace, filtering, scripted transforms |
| `awk` | Text-processing language: field parsing, statistics, reports |
| `grep` (`egrep`/`fgrep`) | Search by regex / extended regex / fixed strings |
| `diff` / `cmp` | Line-by-line / byte-by-byte file comparison |
| `patch` | Apply diff patches |
| `ed` | Classic line editor |
| `vi` | Full-screen editor (search, replace, undo, multiple files) |
| `less` / `more` | Pagers (less supports backward scroll and search) |

### Encoding, checksums & conversion

| Applet | Purpose |
|---|---|
| `base32` / `base64` | Base32/Base64 encode/decode |
| `uuencode` / `uudecode` | UUencode/decode |
| `md5sum` / `sha1sum` / `sha256sum` / `sha384sum` / `sha512sum` / `sha3sum` | Hash checksums |
| `cksum` / `crc32` / `sum` | POSIX checksum / CRC32 / BSD sum |
| `dos2unix` / `unix2dos` | Line-ending conversion |
| `iconv` | Charset conversion |

### Archives & compression

| Applet | Purpose |
|---|---|
| `tar` | Archive/create/extract (auto-detects compression) |
| `cpio` | cpio archives (initramfs, etc.) |
| `ar` | Static library / archive maintenance |
| `gzip`/`gunzip` (`zcat`) | gzip compression/decompression |
| `bzip2`/`bunzip2` (`bzcat`) | bzip2 compression/decompression |
| `xz`/`unxz` (`xzcat`) | xz compression/decompression |
| `lzma`/`unlzma` (`lzcat`), `lzop`/`unlzop` (`lzopcat`) | Other LZ-family codecs |
| `uncompress` | Legacy .Z decompression |
| `unzip` | ZIP extraction (common compression algorithms) |
| `dpkg` / `dpkg-deb` | Debian package (.deb) query/extract |
| `rpm` / `rpm2cpio` | RPM package query / convert to cpio stream |

### Processes & system

| Applet | Purpose |
|---|---|
| `ps` (`minips`) | List processes (task snapshot on Windows) |
| `kill` | Terminate a process by PID |
| `killall` | Terminate processes by name |
| `killall5` | Terminate all processes except self (Linux only) |
| `pgrep` / `pkill` | Match processes by name/attribute: find / signal |
| `pidof` | Find PIDs by name |
| `uptime` | System uptime and load |
| `free` | Memory usage (system info on Windows) |
| `watch` | Run a command periodically and refresh output |
| `time` | Measure command execution time |
| `nproc` | Number of CPU cores |
| `uname` (`arch`) | System/kernel information |
| `hostname` / `dnsdomainname` | Host name / DNS domain |
| `id` / `groups` / `whoami` / `logname` | Current user identity (emulated on Windows) |
| `tty` / `ttysize` | Terminal device name / terminal size |
| `date` / `cal` | Date and time (formatting, setting) / calendar |
| `rdate` | Set time from a remote time server |
| `df` / `du` | Disk free space / directory usage |
| `who` / `users` / `w` | Logged-in users / sessions and load |
| `hostid` | Host ID (Linux only) |

### Networking

| Applet | Purpose |
|---|---|
| `wget` | HTTP/HTTPS/FTP downloader (resume, auth, HTTPS) |
| `nc` (`netcat`) | TCP/UDP swiss army knife: port tests, simple server/client, data transfer |
| `telnet` | Interactive Telnet client |
| `tftp` / `tftpd` | TFTP client / simple TFTP server |
| `ftpget` / `ftpput` | Scriptable (non-interactive) FTP download/upload |
| `httpd` ★ | Embedded HTTP server (static files, CGI, auth, gzip) |
| `nslookup` | DNS queries |
| `whois` | WHOIS domain/IP lookup |
| `ipcalc` | IP/subnet calculator (mask, network, broadcast) |
| `pscan` | Simple port scanner |
| `ssl_client` | Raw TLS/SSL connection (used by the built-in TLS implementation) |

### Math & logic

| Applet | Purpose |
|---|---|
| `bc` / `dc` | Arbitrary-precision calculator (interactive/script) / RPN calculator |
| `expr` | Evaluate expressions (arithmetic, strings, comparison) |
| `factor` | Prime factorization |
| `seq` | Generate arithmetic sequences |
| `printf` | Formatted output |
| `echo` | Print text (`-e` escapes, `-n` no newline) |
| `test` (`[`) | Condition tests (files, strings, numbers) |
| `true` / `false` | Always succeed / always fail |
| `yes` | Print a string repeatedly |
| `sleep` / `usleep` | Sleep for seconds / microseconds |
| `timeout` | Run a command with a time limit |
| `shuf` | Shuffle input lines randomly |
| `tsort` | Topological sort |

### Environment & shell helpers

| Applet | Purpose |
|---|---|
| `env` | Show/set environment and run a command |
| `printenv` | Print environment variables |
| `export`/`set`/`unset`/`cd` etc. | Shell builtins (see ash) |
| `getopt` | Parse command-line options (for scripts) |
| `which` | Locate an executable in PATH |
| `run-parts` | Run all scripts in a directory in order |
| `pipe_progress` | Progress display for pipes (used internally) |
| `make` (`pdpmake`) | Built-in make (POSIX compatible) |
| `man` | View manual pages |

### Development & debugging helpers

| Applet | Purpose |
|---|---|
| `xxd`/`hexdump`/`od` | Hex dumps (see Text processing) |
| `hexedit` | Interactive hex editor |
| `strings` | Extract printable strings from binaries |
| `stty` | Set terminal parameters |
| `clear` / `reset` | Clear screen / reset terminal |
| `resize` | Print shell command to resize the terminal |
| `script` / `scriptreplay` | Record terminal sessions / replay (reproducibility and auditing) |
| `uuidgen` | Generate UUIDs |
| `rev` | Reverse characters of each line |
| `tree` | Display directory tree |
| `ascii` ★ | Display the ASCII table |
| `bbconfig` | Print the current build configuration |
| `jn` | JSON data processing tool |
| `ts` | Timestamp tool |
| `inotifyd` | File-system event monitoring (Windows notifications) |
| `crond` / `crontab` | Cron daemon / schedule table management |
| `chat` | Serial scripted conversation (dial-up, etc.) |
| `microcom` | Simple serial terminal |
| `rx` | XMODEM receiver |

### Windows-specific

| Applet | Purpose |
|---|---|
| `su` (suw32) ★ | Run a shell or command with administrator privileges (ShellExecute elevation; `-c` command, `-t` test mode) |
| `drop` / `cdrop` / `pdrop` ★ | Lower the process token and run a child with normal privileges |
| `chattr` / `lsattr` | Windows file flags (read-only/system/hidden) |

### Testing & internal

| Applet | Purpose |
|---|---|
| `unit` | Unit-test framework entry (disabled by default) |
| `parse` | Config-parsing debug tool (disabled by default) |

---

## Design notes & hints

- **Standalone shell**: built-in applets are called directly from the shell, no PATH needed;
  set `BB_OVERRIDE_APPLETS` (space-separated applet names) to force the use of external programs
- **Terminal modes**: the best terminal handling is auto-detected; `BB_TERMINAL_MODE=1` forces literal
  ANSI escapes, `BB_TERMINAL_MODE=0` forces Windows console API emulation
- **Device-file emulation**: `/dev/null`, `/dev/tty`, `/dev/zero`, `/dev/urandom` work in redirections
  and as arguments
- **Paths**: always use forward slashes; UNC paths `//host/share/path`; the `-X` shell option prevents
  conversion of backslashes to forward slashes
- **Performance**: adding the busybox process to Windows Security exclusions improves performance
  significantly
- **System requirements**: Windows 7+ / Server 2008 R2+ (UTF-8 variants require Windows 10+)

## More information

- Full applet list and options: `busybox.exe --list`, `busybox.exe <applet> --help`
- Development and maintenance notes: [AGENTS.md](AGENTS.md) (platform-layer architecture, conventions
  for adding/removing applets and the ripple-effect checklist)
