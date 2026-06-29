package shell

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestShellParseAndOr(t *testing.T) {
	line := "echo a && echo b || echo c; echo d"
	parts := parseAndOr(line)
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts, got %d", len(parts))
	}
	if parts[0].cmd != "echo a" || parts[0].op != "&&" {
		t.Errorf("part 0 mismatch: %+v", parts[0])
	}
	if parts[1].cmd != "echo b" || parts[1].op != "||" {
		t.Errorf("part 1 mismatch: %+v", parts[1])
	}
	if parts[2].cmd != "echo c" || parts[2].op != ";" {
		t.Errorf("part 2 mismatch: %+v", parts[2])
	}
	if parts[3].cmd != "echo d" || parts[3].op != "" {
		t.Errorf("part 3 mismatch: %+v", parts[3])
	}
}

func TestShellExecuteLineSemicolonAndOperators(t *testing.T) {
	// Intercept stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run commands
	executeLine("echo hello; echo world")
	executeLine("true && echo yes")
	executeLine("false || echo no")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	expectedLines := []string{"hello", "world", "yes", "no"}
	actualLines := strings.Split(strings.TrimSpace(output), "\n")

	for i, expected := range expectedLines {
		if i >= len(actualLines) {
			t.Fatalf("missing output line %d: expected %q", i, expected)
		}
		actual := strings.TrimSpace(actualLines[i])
		if actual != expected {
			t.Errorf("line %d mismatch: expected %q, got %q", i, expected, actual)
		}
	}
}

func TestShellVariablesAndExpansion(t *testing.T) {
	// Set some variables
	executeLine("myvar=123")
	val := expandVars("$myvar")
	if val != "123" {
		t.Errorf("expected 123, got %q", val)
	}

	// Exit code
	executeLine("true")
	exitVal := expandVars("$?")
	if exitVal != "0" {
		t.Errorf("expected 0, got %q", exitVal)
	}

	// PID
	pidVal := expandVars("$$")
	if pidVal != strconv.Itoa(os.Getpid()) {
		t.Errorf("expected %d, got %q", os.Getpid(), pidVal)
	}
}
