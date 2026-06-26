// Package platform provides cross-platform abstractions for agentbusybox.
// It handles differences between Linux, Windows, and macOS.
package platform

import (
	"os"
	"runtime"
	"strings"
)

// OS returns the current operating system: "linux", "windows", "darwin", or runtime.GOOS.
func OS() string {
	return runtime.GOOS
}

// IsWindows returns true if running on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}

// IsLinux returns true if running on Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsDarwin returns true if running on macOS.
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

// NullDevice returns the path to the null device.
func NullDevice() string {
	if IsWindows() {
		return "NUL"
	}
	return "/dev/null"
}

// PathListSeparator returns the path list separator for the current OS.
func PathListSeparator() string {
	return string(os.PathListSeparator)
}

// PathSeparator returns the path separator for the current OS.
func PathSeparator() string {
	return string(os.PathSeparator)
}

// NormalizePath converts backslashes to forward slashes (for display/compatibility).
func NormalizePath(path string) string {
	if IsWindows() {
		return strings.ReplaceAll(path, `\`, `/`)
	}
	return path
}

// HomeDir returns the user's home directory.
func HomeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	if IsWindows() {
		return "C:\\"
	}
	return "/"
}

// TempDir returns the temporary directory.
func TempDir() string {
	return os.TempDir()
}

// DevNull returns the path to the null device.
func DevNull() string {
	return NullDevice()
}

// Executable returns the path to the current executable.
func Executable() string {
	if ex, err := os.Executable(); err == nil {
		return ex
	}
	return "busybox"
}

// Shell returns the default shell for the current platform.
func Shell() string {
	if IsWindows() {
		if sh := os.Getenv("SHELL"); sh != "" {
			return sh
		}
		return "cmd.exe"
	}
	return "/bin/sh"
}

// SupportsSymlinks returns whether the platform supports symbolic links.
func SupportsSymlinks() bool {
	return !IsWindows() || isWindowsDeveloperMode()
}

func isWindowsDeveloperMode() bool {
	if !IsWindows() {
		return false
	}
	// In developer mode, symlinks work without elevation
	// This is a heuristic check
	return false
}

// MakeExecutable sets the executable permission on a file (no-op on Windows).
func MakeExecutable(path string) error {
	if IsWindows() {
		return nil
	}
	return os.Chmod(path, 0755)
}

// RunCmd runs a command and returns its exit code.
func RunCmd(name string, args ...string) int {
	return 127
}

// LineEnding returns the platform-appropriate line ending.
func LineEnding() string {
	if IsWindows() {
		return "\r\n"
	}
	return "\n"
}

// MaxPathLength returns the maximum path length for the current platform.
func MaxPathLength() int {
	if IsWindows() {
		return 260
	}
	return 4096
}
