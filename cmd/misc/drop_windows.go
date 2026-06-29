//go:build windows

package misc

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modadvapi32                    = windows.NewLazySystemDLL("advapi32.dll")
	procSaferCreateLevel           = modadvapi32.NewProc("SaferCreateLevel")
	procSaferComputeTokenFromLevel = modadvapi32.NewProc("SaferComputeTokenFromLevel")
)

const (
	SAFER_SCOPEID_USER       = 2
	SAFER_LEVELID_NORMALUSER = 0x20000
	SAFER_LEVEL_OPEN         = 1
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
			exe = "powershell.exe"
		case "cdrop":
			exe = "cmd.exe"
		case "drop":
			if optShell != "" {
				exe = optShell
			} else {
				exe = "sh.exe"
			}
		default:
			exe = "cmd.exe"
		}
	} else {
		exe = cmdArgs[0]
		runArgs = cmdArgs[1:]
	}

	if fullPath, err := exec.LookPath(exe); err == nil {
		exe = fullPath
	}

	var cmdLineParts []string
	cmdLineParts = append(cmdLineParts, windows.EscapeArg(exe))
	if optCommand != "" {
		if appletName == "cdrop" {
			cmdLineParts = append(cmdLineParts, "/c", optCommand)
		} else {
			cmdLineParts = append(cmdLineParts, "-c", optCommand)
		}
	}
	for _, a := range runArgs {
		cmdLineParts = append(cmdLineParts, windows.EscapeArg(a))
	}
	cmdLine := strings.Join(cmdLineParts, " ")

	var safer windows.Handle
	r1, _, err := procSaferCreateLevel.Call(
		SAFER_SCOPEID_USER,
		SAFER_LEVELID_NORMALUSER,
		SAFER_LEVEL_OPEN,
		uintptr(unsafe.Pointer(&safer)),
		0,
	)
	if r1 == 0 {
		fmt.Fprintf(os.Stderr, "%s: SaferCreateLevel failed: %v\n", appletName, err)
		return 1
	}
	defer windows.CloseHandle(safer)

	var token windows.Token
	r1, _, err = procSaferComputeTokenFromLevel.Call(
		uintptr(safer),
		0,
		uintptr(unsafe.Pointer(&token)),
		0,
		0,
	)
	if r1 == 0 {
		fmt.Fprintf(os.Stderr, "%s: SaferComputeTokenFromLevel failed: %v\n", appletName, err)
		return 1
	}
	defer token.Close()

	mediumSidBytes := []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10, 0x00, 0x20, 0x00, 0x00}
	sid := (*windows.SID)(unsafe.Pointer(&mediumSidBytes[0]))

	var til windows.Tokenmandatorylabel
	til.Label.Sid = sid
	til.Label.Attributes = windows.SE_GROUP_INTEGRITY

	err = windows.SetTokenInformation(
		token,
		windows.TokenIntegrityLevel,
		(*byte)(unsafe.Pointer(&til)),
		uint32(unsafe.Sizeof(til))+windows.GetLengthSid(sid),
	)

	var si windows.StartupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Flags = windows.STARTF_USESTDHANDLES
	si.StdInput = windows.Handle(os.Stdin.Fd())
	si.StdOutput = windows.Handle(os.Stdout.Fd())
	si.StdErr = windows.Handle(os.Stderr.Fd())

	var pi windows.ProcessInformation

	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid command line: %v\n", appletName, err)
		return 1
	}
	exePtr, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid executable path: %v\n", appletName, err)
		return 1
	}

	err = windows.CreateProcessAsUser(
		token,
		exePtr,
		cmdLinePtr,
		nil,
		nil,
		true,
		0,
		nil,
		nil,
		&si,
		&pi,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: CreateProcessAsUser failed: %v\n", appletName, err)
		return 1
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)

	event, err := windows.WaitForSingleObject(pi.Process, windows.INFINITE)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: WaitForSingleObject failed: %v\n", appletName, err)
		return 1
	}
	if event == windows.WAIT_OBJECT_0 {
		var code uint32
		if err := windows.GetExitCodeProcess(pi.Process, &code); err == nil {
			return int(code)
		}
	}

	return 1
}
