(workflow "busybox-go-full-implementation"
  (concurrency 3)

  (phase "research"
    (parallel
      (agent "research-coreutils"
        :mode "plan"
        :tools '("read" "grep" "find" "bash")
        :max-iterations 120
        :prompt (concat
          "Analyze the busybox-w32 coreutils category at /home/free/src/busybox-w32/coreutils/\n"
          "For each .c file, extract: applet name, all flags, key behavior, libbb deps.\n"
          "Output: APPLET: <name> | FLAGS: <list> | SOURCE: <file> | LINES: <count>\n"
          "Focus on: basename,cat,chmod,chown,chroot,cksum,comm,cp,cut,date,dd,df,dirname,dos2unix,du,echo,env,expand,expr,factor,fold,head,hostid,id,install,join,link,ln,logname,ls,md5sum,mkdir,mkfifo,mknod,mv,nl,nproc,od,paste,printenv,printf,pwd,readlink,realpath,rm,rmdir,seq,shred,shuf,sleep,sort,split,stat,stty,sum,sync,tac,tail,tee,touch,tr,true,truncate,tty,uname,unexpand,uniq,unlink,uptime,users,usleep,uudecode,uuencode,wc,who,whoami,yes,base32,base64,crc32,arch\n"
          "Also check /home/free/src/agentbusybox/cmd/coreutils/. Return COMPLETE analysis."))

      (agent "research-textproc"
        :mode "plan"
        :tools '("read" "grep" "find" "bash")
        :max-iterations 120
        :prompt (concat
          "Analyze busybox-w32 text processing:\n"
          "- /home/free/src/busybox-w32/coreutils/ (grep,sed,diff,cmp,strings,xargs)\n"
          "- /home/free/src/busybox-w32/editors/ (awk,vi,ed,patch,sed)\n"
          "- /home/free/src/busybox-w32/findutils/ (find,grep,xargs)\n"
          "For each: name, flags, complexity (simple/medium/complex).\n"
          "Also check /home/free/src/agentbusybox/cmd/textproc/ and cmd/editors/. Return COMPLETE."))

      (agent "research-networking"
        :mode "plan"
        :tools '("read" "grep" "find" "bash")
        :max-iterations 120
        :prompt (concat
          "Analyze busybox-w32 networking at /home/free/src/busybox-w32/networking/\n"
          "For each: name, flags, behavior, root-required.\n"
          "Focus: arp,arping,brctl,dnsd,ether-wake,ftpd,ftpget,ftpput,httpd,ifconfig,ifplugd,inetd,ip,ipcalc,nc,netstat,nslookup,ntpd,ping,ping6,route,tcpsvd,telnet,telnetd,tftp,traceroute,wget,whois,zcip\n"
          "Check /home/free/src/agentbusybox/cmd/networking/. Return COMPLETE."))

      (agent "research-procutils"
        :mode "plan"
        :tools '("read" "grep" "find" "bash")
        :max-iterations 120
        :prompt (concat
          "Analyze busybox-w32:\n"
          "- /home/free/src/busybox-w32/procps/ (ps,top,free,uptime)\n"
          "- /home/free/src/busybox-w32/init/ (init,halt,reboot)\n"
          "- /home/free/src/busybox-w32/sysklogd/ (syslogd,klogd,logger,logread)\n"
          "For each: name, flags, behavior, platform (linux/portable).\n"
          "Check /home/free/src/agentbusybox/cmd/process/,cmd/init/,cmd/sysklogd/. Return COMPLETE."))

      (agent "research-misc"
        :mode "plan"
        :tools '("read" "grep" "find" "bash")
        :max-iterations 120
        :prompt (concat
          "Analyze busybox-w32 misc:\n"
          "- /home/free/src/busybox-w32/miscutils/\n"
          "- /home/free/src/busybox-w32/archival/\n"
          "- /home/free/src/busybox-w32/util-linux/\n"
          "- /home/free/src/busybox-w32/loginutils/\n"
          "- /home/free/src/busybox-w32/modutils/\n"
          "- /home/free/src/busybox-w32/e2fsprogs/\n"
          "- /home/free/src/busybox-w32/runit/\n"
          "- /home/free/src/busybox-w32/selinux/\n"
          "- /home/free/src/busybox-w32/console-tools/\n"
          "- /home/free/src/busybox-w32/debianutils/\n"
          "- /home/free/src/busybox-w32/printutils/\n"
          "- /home/free/src/busybox-w32/mailutils/\n"
          "For each: name, flags, behavior. SKIP disk tools: fdisk,mkfs.*,fsck.*,tune2fs,blkid,blockdev,hdparm,fdformat,fstrim,fsfreeze,mke2fs,e2label,findfs,mkswap,losetup\n"
          "Check /home/free/src/agentbusybox/cmd/. Return COMPLETE."))))

  (phase "gap-analysis"
    (agent "gap-analyzer"
      :mode "plan"
      :tools '("read" "grep" "find" "bash")
      :max-iterations 150
      :prompt (concat
        (results "research")
        "\n\nPerform comprehensive gap analysis.\n"
        "For EACH busybox applet: 1.Already in agentbusybox? 2.Full or stub? 3.Missing? 4.Disk-related? 5.Complexity?\n"
        "Categories: A=ALREADY_DONE B=STUB_NEEDS_WORK C=MISSING_NEED_IMPLEMENT D=SKIP_DISK E=SKIP_PLATFORM\n"
        "Output table: APPLET|CATEGORY|STATUS|COMPLEXITY|PRIORITY|NOTES sorted by priority.\n"
        "Include count per status. Return COMPLETE.")))

  (phase "implementation-plan"
    (agent "planner"
      :mode "plan"
      :tools '("read" "grep" "find" "bash")
      :max-iterations 120
      :prompt (concat
        (result "gap-analysis.gap-analyzer")
        "\n\nCreate detailed implementation plan.\n"
        "For each applet needing work (B or C): Go package, function signature func runXxx(args []string) int, flags, LOC estimate, deps.\n"
        "Batch 1: Simple (<50 LOC), Batch 2: Medium (50-200), Batch 3: Complex (>200), Batch 4: Stub rewrites.\n"
        "Per batch: target package, files, applet list, init() registration.\n"
        "Return COMPLETE plan.")))

  (phase "implement-batch1-simple-coreutils"
    (parallel
      (agent "impl-coreutils-batch1a"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\n\nImplement simple coreutils in /home/free/src/agentbusybox/cmd/coreutils/.\n"
          "RULES: Pure Go, NO exec.Command, No new deps beyond golang.org/x/sys and golang.org/x/net.\n"
          "applet.Register() in init(). args[0]=name, parse flags manually.\n"
          "Read /home/free/src/busybox-w32/ reference. Build after each: go build -o bin/agentbusybox .\n"
          "Write complete working implementations."))

      (agent "impl-coreutils-batch1b"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement simple text/file/find applets in cmd/textproc/,cmd/fileutil/,cmd/findutils/.\n"
          "RULES: Pure Go, NO exec.Command, No new deps.\n"
          "Read busybox source. Build after each."))))

  (phase "implement-batch2-medium"
    (parallel
      (agent "impl-networking"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement networking applets in cmd/networking/. Use Go net package.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Read busybox networking/. Build after."))

      (agent "impl-process-syslog"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement process/syslog/init applets in cmd/process/,cmd/sysklogd/,cmd/init/.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Read /proc. Build after."))

      (agent "impl-util-linux"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement util-linux in cmd/util-linux/. SKIP disk tools.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Read /proc,/sys. Build after."))

      (agent "impl-login-runit"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement login/runit in cmd/login/,cmd/runit/. Use crypto for auth.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))))

  (phase "implement-batch3-complex"
    (parallel
      (agent "impl-awk"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement complete awk interpreter in cmd/editors/. Read busybox awk.c.\n"
          "Implement: field splitting, BEGIN/END, actions, NR/NF/FS/OFS, string/math/IO functions, arrays.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))

      (agent "impl-sed"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement complete sed interpreter in cmd/textproc/. Read busybox sed.c.\n"
          "Implement: s///g, addresses, commands, hold space, -e, -i.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))

      (agent "impl-shell"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement shell improvements in cmd/shell/. Enhance ash/hush.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))

      (agent "impl-vi"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement vi editor in cmd/editors/. Read busybox vi.c.\n"
          "Implement: command/insert/visual modes, movement, editing, search, file ops.\n"
          "Use Go os/syscall for terminal. NO termbox.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))))

  (phase "implement-batch4-remaining"
    (parallel
      (agent "impl-archival"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement archival in cmd/archival/. Use Go compress/archive.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))

      (agent "impl-misc-remaining"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement remaining misc in cmd/misc/. bc,dc,chat,conspy,hexedit,inotifyd,makedevs.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))

      (agent "impl-console-debian-print-mail"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement console/debian/print/mail/selinux. DO NOT touch e2fsprogs.\n"
          "Targets: cmd/console/,cmd/debianutils/,cmd/printutils/,cmd/mailutils/,cmd/selinux/.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))

      (agent "impl-modutils-e2fs"
        :mode "yolo"
        :tools '("read" "grep" "find" "edit" "write" "bash")
        :max-iterations 250
        :prompt (concat
          (result "implementation-plan.planner")
          "\nImplement modutils + e2fsprogs in cmd/modutils/,cmd/e2fsprogs/. Read /proc/modules.\n"
          "SKIP tune2fs (disk-related). chattr read-only only.\n"
          "RULES: Pure Go, NO exec.Command, No new deps. Build after."))))

  (phase "integration-test"
    (agent "integrator"
      :mode "yolo"
      :tools '("read" "grep" "find" "edit" "write" "bash")
      :max-iterations 200
      :prompt (concat
        "Integrate all new applets into agentbusybox.\n"
        "1.Ensure all registered in init()\n"
        "2.Ensure main.go has blank imports\n"
        "3.Build: cd /home/free/src/agentbusybox && go build -o bin/agentbusybox .\n"
        "4.Fix compilation errors\n"
        "5.Test: go test -v -race ./...\n"
        "6.Fix test failures\n"
        "7.Vet: go vet ./...\n"
        "8.Format: gofmt -w . && goimports -w .\n"
        "9.Verify: ./bin/agentbusybox --list | wc -l\n"
        "10.Test applets: ./bin/agentbusybox echo hello\n"
        "Update CHECKLIST.md. Ensure clean build.")))

  (phase "final-verification"
    (parallel
      (agent "verify-build"
        :mode "yolo"
        :tools '("read" "grep" "find" "bash")
        :max-iterations 100
        :prompt (concat
          "Verify build is clean:\n"
          "1.go build -o bin/agentbusybox .\n"
          "2.go vet ./...\n"
          "3.go test -v -race ./...\n"
          "4.Check binary size: ls -lh bin/agentbusybox\n"
          "5.List applets: ./bin/agentbusybox --list\n"
          "6.Compare with busybox count\n"
          "Report issues."))

      (agent "verify-completeness"
        :mode "plan"
        :tools '("read" "grep" "find" "bash")
        :max-iterations 100
        :prompt (concat
          "Verify completeness:\n"
          "1.Count applets: grep -rn 'applet.Register' cmd/ | wc -l\n"
          "2.Compare with busybox 366\n"
          "3.Check CHECKLIST.md remaining items\n"
          "4.No disk tools implemented\n"
          "5.All research applets accounted for\n"
          "Report completeness percentage.")))))
