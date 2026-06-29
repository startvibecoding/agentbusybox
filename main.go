package main

import (
	"os"

	_ "github.com/agentbusybox/cmd/archival"
	_ "github.com/agentbusybox/cmd/busybox"
	_ "github.com/agentbusybox/cmd/console"
	_ "github.com/agentbusybox/cmd/coreutils"
	_ "github.com/agentbusybox/cmd/debianutils"
	_ "github.com/agentbusybox/cmd/e2fsprogs"
	_ "github.com/agentbusybox/cmd/editors"
	_ "github.com/agentbusybox/cmd/fastutil"
	_ "github.com/agentbusybox/cmd/fileutil"
	_ "github.com/agentbusybox/cmd/findutils"
	_ "github.com/agentbusybox/cmd/init"
	_ "github.com/agentbusybox/cmd/klibc"
	_ "github.com/agentbusybox/cmd/login"
	_ "github.com/agentbusybox/cmd/mailutils"
	_ "github.com/agentbusybox/cmd/misc"
	_ "github.com/agentbusybox/cmd/modutils"
	_ "github.com/agentbusybox/cmd/networking"
	_ "github.com/agentbusybox/cmd/printutils"
	_ "github.com/agentbusybox/cmd/process"
	_ "github.com/agentbusybox/cmd/rootfs"
	_ "github.com/agentbusybox/cmd/runit"
	_ "github.com/agentbusybox/cmd/selinux"
	_ "github.com/agentbusybox/cmd/shell"
	_ "github.com/agentbusybox/cmd/sysklogd"
	_ "github.com/agentbusybox/cmd/textproc"
	_ "github.com/agentbusybox/cmd/util-linux"
	"github.com/agentbusybox/pkg/applet"
)

func main() {
	os.Exit(applet.Dispatch())
}
