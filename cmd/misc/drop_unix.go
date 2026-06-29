//go:build !windows

package misc

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func runDropPlatform(args []string) int {
	var optCommand string
	var optShell string
	cmdArgs := []string{}

	appletName := args[0]

	for i := 1; i < len(args); i++ {
		arg := args[i]
		if arg == "-c" {
			if i+1 < len(args) {
				optCommand = args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "%s: -c requires an argument\n", appletName)
				return 1
			}
		} else if arg == "-s" && appletName == "drop" {
			if i+1 < len(args) {
				optShell = args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "%s: -s requires an argument\n", appletName)
				return 1
			}
		} else {
			cmdArgs = append(cmdArgs, arg)
		}
	}

	var exe string
	var runArgs []string

	if len(cmdArgs) == 0 || optCommand != "" {
		switch appletName {
		case "pdrop":
			exe = "powershell"
			if _, err := exec.LookPath(exe); err != nil {
				exe = "pwsh"
			}
		case "cdrop":
			exe = "sh"
		case "drop":
			if optShell != "" {
				exe = optShell
			} else {
				exe = "sh"
			}
		default:
			exe = "sh"
		}
		if optCommand != "" {
			runArgs = []string{"-c", optCommand}
			runArgs = append(runArgs, cmdArgs...)
		}
	} else {
		exe = cmdArgs[0]
		runArgs = cmdArgs[1:]
	}

	cmd := exec.Command(exe, runArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if os.Getuid() == 0 {
		uid := 65534
		gid := 65534

		if sudoUidStr := os.Getenv("SUDO_UID"); sudoUidStr != "" {
			if u, err := strconv.Atoi(sudoUidStr); err == nil {
				uid = u
			}
		}
		if sudoGidStr := os.Getenv("SUDO_GID"); sudoGidStr != "" {
			if g, err := strconv.Atoi(sudoGidStr); err == nil {
				gid = g
			}
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid: uint32(uid),
				Gid: uint32(gid),
			},
		}

		unprivilegedUser := "nobody"
		if uid != 65534 {
			if user := os.Getenv("SUDO_USER"); user != "" {
				unprivilegedUser = user
			}
		}
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "USER="+unprivilegedUser)
		cmd.Env = append(cmd.Env, "LOGNAME="+unprivilegedUser)
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				return status.ExitStatus()
			}
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", appletName, err)
		return 1
	}
	return 0
}
