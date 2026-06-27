# AgentBusyBox Flag Parity Checklist
# Reference: /home/free/src/busybox-w32
# Status: ✅ = done, ⬜ = pending, 🔄 = partial

## coreutils/
- [] basename: -a -s SUFFIX
- [] cat: -u -n -b -s -E -T -v -e -t -A
- [] chgrp: -R -h -f
- [] chmod: -R -f
- [] chown: -R -h -f
- [] comm: -1 -2 -3
- [] cp: -a -r -R -d -P -f -i -n -l -s -L -H -p -v -u -T -t --archive --force --interactive --no-clobber --link --dereference --no-dereference --recursive --symbolic-link --no-target-directory --target-directory --verbose --update --remove-destination --parents
- [] cut: -b -c -f -d -s
- [] date: -R -s -u -d -r -I -D
- [] dd: if= of= bs= count= skip= seek= conv= iflag= oflag=
- [] df: -a -h -k -P -T
- [] dirname: (no flags)
- [] dos2unix: -d -u
- [] du: -a -s -h -k -l -c -d
- [] echo: -n -e -E
- [] env: -i -0 -u
- [] expand: -t
- [] expr: expressions
- [] factor: (no flags)
- [] false: (no flags)
- [] fold: -b -s -w
- [] head: -n -c -q -v + legacy numeric syntax (-5)
- [] id: -a -G -g -n -r -u
- [] install: -d -D -m -o -g -t
- [] join: -a -e -j -1 -2 -o -t
- [] ln: -s -f -n -b -v
- [] ls: -1 -A -a -C -x -d -l -i -n -g -s -F -p -R -Q -c -t -u -S -X -r -v -L -H -h -T -w -q -k --full-time --group-directories-first (missing: -Z, --color)
- [] mkdir: -m -p -v
- [] mkfifo: -m
- [] mknod: -m
- [] mv: -f -i -n -u -v -T -t -b -S --backup --strip-trailing-slashes --no-clobber --target-directory --update
- [] nice: -n
- [] nl: -b -i -s -v -w -p
- [] nohup: (no flags)
- [] nproc: --all --ignore
- [] od: -a -B -b -c -D -d -e -F -f -H -h -I -i -L -l -O -o -v -X -x
- [] paste: -d -s
- [] printf: format
- [] pwd: -L -P
- [] readlink: -n -f -s -v -q
- [] realpath: (no flags)
- [] rm: -f -i -R -r -v -I --preserve-root --no-preserve-root
- [] rmdir: -p -v
- [] seq: -w -s -f
- [] shred: -f -u -z -n -v -x -s
- [] shuf: -e -i -n -o -r
- [] sleep: N[suffix]
- [] sort: -n -r -u -g -h -M -c -s -z -b -d -f -i -o -k -t
- [] split: -a -b -l -d -x
- [] stat: -L -f -c
- [] stty: -a -g
- [] sum: -r -s
- [] sync: -d -f
- [] tail: -f -c -n -s -v --follow +N syntax -F
- [] tac: (no flags)
- [] tee: -i -a
- [] test: (complex expressions)
- [] timeout: -s -k
- [] touch: -c -d -r -t -a -m -f
- [] tr: -C -c -d -s
- [] true: (no flags)
- [] truncate: -c -s
- [] tty: -s
- [] uname: -a -m -n -r -s -v -p -i -o
- [] unexpand: -t --first-only --all
- [] uniq: -c -d -u -f -s -w -i -z
- [] unlink: (no flags)
- [] uptime: -s
- [] users: [FILE]
- [] uuencode: -m
- [] uudecode: -o
- [] w: [USER]
- [] wc: -l -w -m -c -L
- [] who: -a -H
- [] whoami: (no flags)
- [] yes: (no flags)
- [] base32: -d -i -w
- [] base64: -d -i -w
- [] cksum: (no flags)
- [] crc32: (no flags)

## networking/
- [] arp: show/manipulate
- [] arping: (delegated to system)
- [] brctl: show/commands
- [] dnsd: not implemented
- [] dnsdomainname: (no flags)
- [] ether-wake: not implemented
- [] fakeidentd: not implemented
- [] ftpd: listen
- [] ftpget: -c -v -g -P -p -u -p (missing proper flag parsing)
- [] ftpput: -c -v -p -u -p (not implemented)
- [] hostname: -d -f -i -s -F
- [] httpd: -p -h
- [] ifconfig: -a + interface config
- [] ifup/ifdown: (delegated)
- [] ifplugd: not implemented
- [] ifenslave: not implemented
- [] inetd: not implemented
- [] ip: addr/link/route/rule/neigh/tunnel
- [] ipaddr/iproute/iplink/iprule/ipneigh/iptunnel: (aliases)
- [] ipcalc: -m -n --broadcast --network
- [] nc: -l -4 -6
- [] netstat: -r -a -l -t -u -w -x -e -n -W -p
- [] nslookup: HOST [SERVER]
- [] ntpd: -d -n -q -N -w -l -S -p -k -f -i (missing proper flags)
- [] ping: -c -i -I -s -t -w -W -4 -6
- [] ping6: (same as ping)
- [] pscan: HOST MIN_PORT MAX_PORT
- [] route: add/del/show
- [] slattach: not implemented
- [] ssl_client: not implemented
- [] ssl_server: not implemented
- [] tc: (delegated)
- [] tcpsvd/udpsvd: -h -E -v -c -C -b -u -l (missing flags)
- [] telnet: HOST PORT
- [] telnetd: -h -i -l -p -f -K -b (missing flags)
- [] tftp: -c -g -l -p -r
- [] tftpd: -c -l -r -u -U (not implemented)
- [] traceroute: (delegated)
- [] traceroute6: (delegated)
- [] tunctl: not implemented
- [] vconfig: (delegated)
- [] wget: -O -q -c -S -P -U -T --header --post-data --spider --no-check-certificate
- [] whois: query
- [] zcip: not implemented
- [] udhcpc/udhcpd/dhcprelay/dumpleases/udhcpc6: stubs only

## procps/
- [] free: -h -b -k -m -g
- [] fuser: -k -m -s -v
- [] iostat: (basic)
- [] kill: -l -s -SIG
- [] killall: (delegated)
- [] killall5: -l -s -SIG -o
- [] lsof: (basic Linux)
- [] mpstat: -A -I -P -u
- [] nmeter: basic only
- [] pgrep: -v -l -a -f -x -o -n -e -s -P
- [] pidof: -s -o -x
- [] pkill: -v -l -x -e -P
- [] pmap: -x -q
- [] powertop: not implemented
- [] ps: -o -T -Z -A -a -d -e -f -l
- [] pstree: -p
- [] sysctl: -n -w -e -N -p -a
- [] top: -d -n -b -H -m
- [] uptime: -s
- [] vmstat: -n DELAY COUNT
- [] watch: -d -t -n -x
- [] pwdx: PID
- [] smemcap: not implemented

## editors/
- [] awk: -F -v (missing: proper field refs $1..$NF, NR, NF, FS, BEGIN/END, conditionals, loops)
- [] cmp: -l -s -n
- [] diff: -a -b -B -d -i -N -q -r -T -s -t -w -L -S -U
- [] ed: -p -s
- [] patch: -p -i -R -N -f -E -g -d
- [] sed: -n -e -f -i -iSFX -r -E --in-place
- [] vi: -c -R -H

## findutils/
- [] find: -name -iname -type -perm -mtime -newer -size -maxdepth -exec -delete -print
- [] grep: -H -h -n -l -L -o -q -v -s -r -R -i -w -x -m -c -A -B -C -e -f -F -E -a -I --include --exclude
- [] egrep: (alias grep -E)
- [] fgrep: (alias grep -F)
- [] xargs: -n -d -I -0 -p -t

## archival/
- [] ar: -d -p -q -r -t -x
- [] bunzip2: -c -f
- [] bzcat: (no flags)
- [] bzip2: -c -d -z -k -q -v -s -f -t -1..9 (delegated to system)
- [] cpio: -o -i -t -p (missing: -0 -a -c -v -A -B -L -V -C -H -M -O -E -R)
- [] dpkg: -i -l -P -r -C -u (delegated)
- [] dpkg-deb: -c -e -x -f -I (delegated)
- [] gzip: -c -d -f -k -n -q -r -v -S -1..9
- [] gunzip: -c -f -t -n -v
- [] lzma: -d -f -t (delegated)
- [] lzop: -c -f -v -d -D -U -k -q -1..9 (delegated)
- [] rpm: -i -q -p -l -c -d -v (delegated)
- [] rpm2cpio: (delegated)
- [] tar: c/t/x -z -j -f -v -C -x
- [] uncompress: -c -f (delegated)
- [] unlzop: (delegated)
- [] unxz: -c -d -f -t -k (delegated)
- [] unzip: -d -l -n -o -p -t -q -x -j -v -K
- [] xz: -c -d -f -t -k (delegated)
- [] zcat: (no flags)
- [] zip: (basic)

## util-linux/
- [] acpid: not implemented
- [] blkdiscard: -o -l -s -f (delegated)
- [] blkid: -s -g
- [] blockdev: (delegated)
- [] cal: -j -m -y
- [] chrt: -m -o -f -d -r -b -i -R -S -p
- [] dmesg: -c -s -n -r
- [] eject: -t -T -s
- [] fallocate: -l -o (stub)
- [] fatattr: not implemented
- [] fbset: (delegated)
- [] fdformat: not implemented
- [] fdflush: not implemented
- [] fdisk: -b -C -H -S -c -h -s -u
- [] findfs: (delegated)
- [] flock: -s -x -n -u -w
- [] fstrim: -o -l -m -v (not implemented)
- [] fsfreeze: --freeze --unfreeze (not implemented)
- [] getopt: -o -l
- [] hexdump: -b -c -C -d -o -v -x -e -f -n -s
- [] hwclock: -s -w -u -l -f
- [] ionice: -n -c -p -t
- [] ipcrm: -q -m -s -Q -M -S
- [] ipcs: -a -i -q -m -s -c -l -p -u -t
- [] last: -f -W
- [] losetup: -o -f -c -d -r -a
- [] lsattr: -R -v -a (delegated)
- [] lsblk: -a
- [] lspci: -m -k -v -n -d
- [] lsusb: (no flags)
- [] mdev: -s -d -S (not implemented)
- [] mesg: y|n
- [] mkfs_*: (delegated)
- [] mkswap: -L
- [] more: -d -e -f -l -s -u
- [] mount: -a -r -w -o -t
- [] mountpoint: -q -d -x -n
- [] nsenter: -t -m -n -i -u -p -U -r -w
- [] pivot_root: (requires CAP_SYS_ADMIN)
- [] rdate: -s -p (not implemented)
- [] readprofile: -M -m -p -n -a -b -s -i -r -v (delegated)
- [] renice: -n -p -g -u
- [] rev: (no flags)
- [] rdev: (stub)
- [] rtcwake: -a -m -d -l -u -t -s (delegated)
- [] script: -a -f -q -t -c -I
- [] scriptreplay: -t -s
- [] setarch: ARCH [PROGRAM]
- [] setpriv: --dump --nnp --inh-caps --ambient-caps --bounding-set --rgid --egid --ruid --euid --securebits --selinux-label --apparmor-profile --reset-env (stub)
- [] setsid: -c
- [] switch_root: -c -r (not implemented)
- [] taskset: -p -c
- [] umount: -a -r -d -D -f -l -n -t
- [] unshare: -i -m -n -p -u -U -C --fork --mount-proc -w
- [] uuidgen: -r
- [] wall: [FILE]

## misc/
- [] adjtimex: -q -o -f -p -t -m -s -S -T -e -c -i -r (delegated)
- [] bbconfig: (no flags)
- [] beep: -f -l -d -r -n
- [] bc: -w -v -s -q -l -i (stub only, no proper expression eval)
- [] chat: -v -V -S -s -E (not implemented)
- [] conspy: not implemented
- [] crond: -b -l -L -f -d -S -c (missing flags)
- [] crontab: -u -c -l -r -e -d (missing flags)
- [] dc: -e -f (stub only)
- [] devmem: ADDRESS [WIDTH [VALUE]]
- [] hdparm: (delegated, no native flags)
- [] hexedit: not implemented
- [] iconv: -f -t -o -l -c
- [] inotifyd: not implemented
- [] less: (basic pager)
- [] lsscsi: (delegated)
- [] makedevs: not implemented
- [] make: (delegated)
- [] man: -a -w
- [] microcom: -X -s -d -t (delegated)
- [] mt: not implemented
- [] nanddump/nandwrite: not implemented
- [] partprobe: not implemented
- [] readahead: not implemented
- [] rfkill: (delegated)
- [] runlevel: [UTMP_FILE]
- [] rx: not implemented
- [] seedrng: not implemented
- [] setfattr: -n -v
- [] setserial: (delegated)
- [] strings: -a -f -o -n -t
- [] time: -v -p -a -o
- [] tree: -L
- [] ts: -i -s
- [] ttysize: (no flags)
- [] ubi*: not implemented
- [] watchdog: -t -T
- [] volname: (delegated)
- [] devfsd: not implemented
- [] getfattr: -d -e -n -m
- [] setfattr: -n -v
- [] i2c*: (delegated)
- [] flash_*: not implemented (hardware)
- [] rfkill: (delegated)

## login/
- [] addgroup: -g -S
- [] adduser: -h -s -G -S -H -u -D
- [] chpasswd: -e -m -c -R (stub)
- [] cryptpw/mkpasswd: -a -m -s -P -S (stub)
- [] delgroup: [USER] GROUP
- [] deluser: --remove-home
- [] getty: -L -H -l -f -I -t -m (not implemented)
- [] login: -f -h -p
- [] passwd: -a -l -u -d
- [] su: -l -m -p -c -s
- [] sulogin: -p -t (stub)
- [] vlock: -a

## shell/
- [] ash/sh/hush: -c SCRIPT + interactive
- [] cttyhack: [COMMAND]

## init/
- [] init: /etc/inittab
- [] halt: -d -n -f -w -i (missing flags)
- [] poweroff: -d -n -f -w -i (missing flags)
- [] reboot: -d -n -f -w -i (missing flags)
- [] bootchartd: start|stop|init (stub)
- [] linuxrc: (no flags)

## runit/
- [] chpst: -v -P -0 -1 -2 -u -U -b -e -/ -n -l -L -m -d -o -p -f -c (stub)
- [] envdir: DIR PROG
- [] envuidgid: -e USER PROG (stub)
- [] runsv: DIR
- [] runsvdir: -P -s -p DIR (stub)
- [] setuidgid: USER PROG
- [] softlimit: -m -d -o -p -f -c PROG (stub)
- [] sv: -v -w command SERVICE
- [] svc: -v -w -u -d -o -once -hup -int -term -kill -exit
- [] svlogd: -t -v -r -R -l -b -p -a -e -f (not implemented)
- [] svok: DIR

## sysklogd/
- [] klogd: -c -n (delegated)
- [] logger: -s -t -p
- [] logread: -f -F (stub)
- [] syslogd: -n -O -l -S -s -b -R -D -f -K -a (delegated)

## e2fsprogs/
- [] chattr: -R -f -v -p [+-=flags] (delegated)
- [] fsck: -A -N -P -R -T -V -C -t (delegated)
- [] lsattr: -R -a -d -l -v (delegated)
- [] tune2fs: many flags (delegated)

## modutils/
- [] insmod: MODULE [OPTIONS]
- [] lsmod: (no flags, reads /proc/modules)
- [] rmmod: -w -f
- [] modprobe: -a -l -r -q -v -s -D -b
- [] modinfo: -a -d -l -p -0 -F
- [] depmod: -n -b

## console/
- [] chvt: N
- [] clear: (no flags)
- [] deallocvt: [N]
- [] dumpkmap: (delegated)
- [] fgconsole: (no flags)
- [] kbd_mode: -a -k -s -u (delegated)
- [] loadfont: (delegated)
- [] loadkmap: (delegated)
- [] openvt: -c -s -w -l
- [] reset: (no flags)
- [] resize: (no flags)
- [] setconsole: -r -v (delegated)
- [] setfont: (delegated)
- [] setkeycodes: (delegated)
- [] setlogcons: (delegated)
- [] showkey: -a -k -s (delegated)

## debianutils/
- [] run-parts: -a -u --reverse --test --exit-on-error --list
- [] start-stop-daemon: -K -S -B -x -a -d -n -p -s -u -c -r -m -q
- [] pipe_progress: (no flags)

## findutils/
- [] find: -name -iname -type -perm -mtime -newer -size -maxdepth -exec -delete -print
- [] grep: all BusyBox flags
- [] egrep: alias
- [] fgrep: alias
- [] xargs: -n -d -I -0 -p -t

## NEW: rootfs/
- [] rootfs: --tar --tar.gz --minimal --bin
