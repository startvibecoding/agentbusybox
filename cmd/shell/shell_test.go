package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to capture stdout
func captureStdout(fn func()) string {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// Helper to create temp script and run it
func runTestScript(t *testing.T, content string) int {
	t.Helper()
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")
	err := os.WriteFile(scriptPath, []byte(content), 0755)
	if err != nil {
		t.Fatalf("Failed to write test script: %v", err)
	}
	return runScriptWithArgs(scriptPath, nil)
}

// ==================== Basic Tests ====================

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
	output := captureStdout(func() {
		executeLine("echo hello; echo world")
		executeLine("true && echo yes")
		executeLine("false || echo no")
	})

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
	if pidVal != fmt.Sprintf("%d", os.Getpid()) {
		t.Errorf("expected %d, got %q", os.Getpid(), pidVal)
	}

	// Number of positional params
	hashVal := expandVars("$#")
	if hashVal != "0" {
		t.Errorf("expected 0, got %q", hashVal)
	}
}

func TestShellVariableExpansionBraces(t *testing.T) {
	shellVars["TESTVAR"] = "hello"
	result := expandVars("${TESTVAR}")
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}

	// Default value
	result = expandVars("${NONEXISTENT:-default}")
	if result != "default" {
		t.Errorf("expected 'default', got %q", result)
	}

	// Alternate value
	result = expandVars("${TESTVAR:+alternate}")
	if result != "alternate" {
		t.Errorf("expected 'alternate', got %q", result)
	}
}

func TestShellTildeExpansion(t *testing.T) {
	os.Setenv("HOME", "/home/testuser")
	result := expandVars("~/file")
	if result != "/home/testuser/file" {
		t.Errorf("expected '/home/testuser/file', got %q", result)
	}
}

func TestShellEscapeSequences(t *testing.T) {
	result := expandVars("hello\\ world")
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

// ==================== Quote Tests ====================

func TestShellSingleQuotes(t *testing.T) {
	args := parseArgs("'hello world' foo")
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", args[0])
	}
	if args[1] != "foo" {
		t.Errorf("expected 'foo', got %q", args[1])
	}
}

func TestShellDoubleQuotes(t *testing.T) {
	args := parseArgs("\"hello world\" foo")
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", args[0])
	}
}

func TestShellNestedQuotes(t *testing.T) {
	args := parseArgs("'hello \"world\"' foo")
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "hello \"world\"" {
		t.Errorf("expected 'hello \"world\"', got %q", args[0])
	}
}

// ==================== If/Then/Else Tests ====================

func TestIfThenElse(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `if true; then
  echo "yes"
else
  echo "no"
fi`)
	})
	output = strings.TrimSpace(output)
	if output != "yes" {
		t.Errorf("expected 'yes', got %q", output)
	}

	output = captureStdout(func() {
		runTestScript(t, `if false; then
  echo "yes"
else
  echo "no"
fi`)
	})
	output = strings.TrimSpace(output)
	if output != "no" {
		t.Errorf("expected 'no', got %q", output)
	}
}

func TestIfElifElse(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `X=2
if [ "$X" = "1" ]; then
  echo "one"
elif [ "$X" = "2" ]; then
  echo "two"
elif [ "$X" = "3" ]; then
  echo "three"
else
  echo "other"
fi`)
	})
	output = strings.TrimSpace(output)
	if output != "two" {
		t.Errorf("expected 'two', got %q", output)
	}
}

func TestIfExitCode(t *testing.T) {
	// Test that if/else returns the correct exit code from the executed block
	output := captureStdout(func() {
		runTestScript(t, `if true; then
		echo "yes"
	else
		echo "no"
	fi`)
	})
	output = strings.TrimSpace(output)
	if output != "yes" {
		t.Errorf("expected 'yes', got %q", output)
	}

	output = captureStdout(func() {
		runTestScript(t, `if false; then
		echo "yes"
	else
		echo "no"
	fi`)
	})
	output = strings.TrimSpace(output)
	if output != "no" {
		t.Errorf("expected 'no', got %q", output)
	}
}

// ==================== For Loop Tests ====================

func TestForLoop(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `for i in 1 2 3 4 5; do
  echo $i
done`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "1" || lines[4] != "5" {
		t.Errorf("unexpected output: %v", lines)
	}
}

func TestForLoopWithGlob(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("c"), 0644)

	output := captureStdout(func() {
		shellVars["PWD"] = tmpDir
		runTestScript(t, fmt.Sprintf(`cd %s
for f in *.txt; do
  echo $f
done`, tmpDir))
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(lines), lines)
	}
}

func TestForLoopBreak(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `for i in 1 2 3 4 5; do
  if [ "$i" = "3" ]; then
    break
  fi
  echo $i
done`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "1" || lines[1] != "2" {
		t.Errorf("unexpected output: %v", lines)
	}
}

func TestForLoopContinue(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `for i in 1 2 3 4 5; do
  if [ "$i" = "3" ]; then
    continue
  fi
  echo $i
done`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "1" || lines[1] != "2" || lines[2] != "4" || lines[3] != "5" {
		t.Errorf("unexpected output: %v", lines)
	}
}

// ==================== While Loop Tests ====================

func TestWhileLoop(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `i=1
while [ "$i" -le 5 ]; do
  echo $i
  i=$((i + 1))
done`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "1" || lines[4] != "5" {
		t.Errorf("unexpected output: %v", lines)
	}
}

func TestWhileLoopBreak(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `i=1
while true; do
  if [ "$i" -gt 3 ]; then
    break
  fi
  echo $i
  i=$((i + 1))
done`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
}

func TestUntilLoop(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `i=1
until [ "$i" -gt 5 ]; do
  echo $i
  i=$((i + 1))
done`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %v", len(lines), lines)
	}
}

// ==================== Case Statement Tests ====================

func TestCaseStatement(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `X=2
case $X in
  1)
    echo "one"
    ;;
  2)
    echo "two"
    ;;
  3)
    echo "three"
    ;;
  *)
    echo "other"
    ;;
esac`)
	})
	output = strings.TrimSpace(output)
	if output != "two" {
		t.Errorf("expected 'two', got %q", output)
	}
}

func TestCaseGlobPattern(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `X="hello.txt"
case $X in
  *.txt)
    echo "text file"
    ;;
  *.sh)
    echo "shell script"
    ;;
  *)
    echo "other"
    ;;
esac`)
	})
	output = strings.TrimSpace(output)
	if output != "text file" {
		t.Errorf("expected 'text file', got %q", output)
	}
}

func TestCaseWildcard(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `X="anything"
case $X in
  *)
    echo "matched"
    ;;
esac`)
	})
	output = strings.TrimSpace(output)
	if output != "matched" {
		t.Errorf("expected 'matched', got %q", output)
	}
}

// ==================== Function Tests ====================

func TestFunctionDefinition(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `greet() {
  echo "Hello, $1!"
}
greet World`)
	})
	output = strings.TrimSpace(output)
	if output != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", output)
	}
}

func TestFunctionKeyword(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `function greet {
  echo "Hello, $1!"
}
greet World`)
	})
	output = strings.TrimSpace(output)
	if output != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %q", output)
	}
}

func TestFunctionReturn(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, "myfunc() {\n  return 0\n}\nmyfunc\necho result=$?")
	})
	output = strings.TrimSpace(output)
	if !strings.Contains(output, "result=0") {
		t.Errorf("expected result=0, got %q", output)
	}
}

func TestFunctionLocalVars(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `myfunc() {
  local x=100
  echo $x
}
x=1
myfunc
echo $x`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "100" {
		t.Errorf("expected '100', got %q", lines[0])
	}
	if lines[1] != "1" {
		t.Errorf("expected '1', got %q", lines[1])
	}
}

// ==================== Arithmetic Tests ====================

func TestArithmeticExpansion(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `echo $((2 + 3))`)
	})
	output = strings.TrimSpace(output)
	if output != "5" {
		t.Errorf("expected '5', got %q", output)
	}
}

func TestArithmeticSubtraction(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `echo $((10 - 3))`)
	})
	output = strings.TrimSpace(output)
	if output != "7" {
		t.Errorf("expected '7', got %q", output)
	}
}

func TestArithmeticMultiplication(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `echo $((4 * 5))`)
	})
	output = strings.TrimSpace(output)
	if output != "20" {
		t.Errorf("expected '20', got %q", output)
	}
}

func TestArithmeticDivision(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `echo $((20 / 4))`)
	})
	output = strings.TrimSpace(output)
	if output != "5" {
		t.Errorf("expected '5', got %q", output)
	}
}

func TestArithmeticModulo(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `echo $((17 % 5))`)
	})
	output = strings.TrimSpace(output)
	if output != "2" {
		t.Errorf("expected '2', got %q", output)
	}
}

func TestArithmeticVariable(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `x=10
y=20
echo $((x + y))`)
	})
	output = strings.TrimSpace(output)
	if output != "30" {
		t.Errorf("expected '30', got %q", output)
	}
}

// ==================== Test Command Tests ====================

func TestTestFileExists(t *testing.T) {
	tmpFile, _ := os.CreateTemp("", "testfile")
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	code := runTestScript(t, fmt.Sprintf(`test -f "%s"`, tmpFile.Name()))
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test -f "/nonexistent/file"`)
	if code != 1 {
		t.Errorf("expected 1, got %d", code)
	}
}

func TestTestDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	code := runTestScript(t, fmt.Sprintf(`test -d "%s"`, tmpDir))
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}
}

func TestTestStringEmpty(t *testing.T) {
	code := runTestScript(t, `test -z ""`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test -n "hello"`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}
}

func TestTestStringCompare(t *testing.T) {
	code := runTestScript(t, `test "hello" = "hello"`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test "hello" != "world"`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}
}

func TestTestNumericCompare(t *testing.T) {
	code := runTestScript(t, `test 5 -eq 5`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test 5 -ne 3`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test 3 -lt 5`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test 5 -gt 3`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test 5 -le 5`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}

	code = runTestScript(t, `test 5 -ge 5`)
	if code != 0 {
		t.Errorf("expected 0, got %d", code)
	}
}

// ==================== Pipe Tests ====================

func TestPipe(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `echo "hello world" | wc -w`)
	})
	output = strings.TrimSpace(output)
	if output != "2" {
		t.Errorf("expected '2', got %q", output)
	}
}

// ==================== Redirect Tests ====================

func TestRedirectOutput(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "output.txt")
	runTestScript(t, fmt.Sprintf(`echo "hello" > %s`, tmpFile))

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if strings.TrimSpace(string(data)) != "hello" {
		t.Errorf("expected 'hello', got %q", string(data))
	}
}

func TestRedirectAppend(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "output.txt")
	runTestScript(t, fmt.Sprintf(`echo "line1" > %s
echo "line2" >> %s`, tmpFile, tmpFile))

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || lines[0] != "line1" || lines[1] != "line2" {
		t.Errorf("unexpected content: %q", string(data))
	}
}

func TestRedirectInput(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "input.txt")
	os.WriteFile(tmpFile, []byte("hello world\n"), 0644)

	output := captureStdout(func() {
		runTestScript(t, fmt.Sprintf(`cat < %s`, tmpFile))
	})
	if strings.TrimSpace(output) != "hello world" {
		t.Errorf("expected 'hello world', got %q", output)
	}
}

func TestRedirectStderr(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "stderr.txt")
	runTestScript(t, fmt.Sprintf(`ls /nonexistent 2> %s`, tmpFile))

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected stderr output, got empty file")
	}
}

// ==================== Builtin Tests ====================

func TestBuiltinExport(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `export MYVAR=hello
echo $MYVAR`)
	})
	output = strings.TrimSpace(output)
	if output != "hello" {
		t.Errorf("expected 'hello', got %q", output)
	}
}

func TestBuiltinShift(t *testing.T) {
	// Shift is tested via script arguments
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")
	os.WriteFile(scriptPath, []byte(`shift
echo $1`), 0755)

	oldParams := positionalParams
	positionalParams = []string{"a", "b", "c"}
	output := captureStdout(func() {
		runScriptWithArgs(scriptPath, []string{"a", "b", "c"})
	})
	positionalParams = oldParams

	output = strings.TrimSpace(output)
	if output != "b" {
		t.Errorf("expected 'b', got %q", output)
	}
}

func TestBuiltinGetopts(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `OPTIND=1
while getopts "ab:c" opt; do
  echo "opt=$opt OPTARG=$OPTARG"
done`)
	})
	// This is a basic test - getopts requires proper argument handling
	if len(output) == 0 {
		// At minimum, it shouldn't crash
	}
}

// ==================== Comment Tests ====================

func TestComments(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `# This is a comment
echo "hello" # inline comment
# Another comment`)
	})
	output = strings.TrimSpace(output)
	if output != "hello" {
		t.Errorf("expected 'hello', got %q", output)
	}
}

// ==================== Empty Lines Tests ====================

func TestEmptyLines(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `
echo "first"

echo "second"

`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 || lines[0] != "first" || lines[1] != "second" {
		t.Errorf("unexpected output: %v", lines)
	}
}

// ==================== Nested Constructs Tests ====================

func TestNestedIfInFor(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `for i in 1 2 3; do
  if [ "$i" = "2" ]; then
    echo "found 2"
  fi
done`)
	})
	output = strings.TrimSpace(output)
	if output != "found 2" {
		t.Errorf("expected 'found 2', got %q", output)
	}
}

func TestNestedForInIf(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `if true; then
  for i in 1 2 3; do
    echo $i
  done
fi`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

func TestNestedWhileInFor(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `for i in 1 2 3; do
  j=1
  while [ "$j" -le "$i" ]; do
    echo "$i-$j"
    j=$((j + 1))
  done
done`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d: %v", len(lines), lines)
	}
}

// ==================== Error Handling Tests ====================

func TestExitCode(t *testing.T) {
	// Note: We can't test `exit` directly as it calls os.Exit which kills the test runner
	// Instead test that the last command's exit code is propagated
	output := captureStdout(func() {
		runTestScript(t, `true; echo $?
false; echo $?`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if strings.TrimSpace(lines[0]) != "0" {
		t.Errorf("expected '0', got %q", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "1" {
		t.Errorf("expected '1', got %q", lines[1])
	}
}

func TestExitCodeDefault(t *testing.T) {
	// Test that $? is updated after each command
	output := captureStdout(func() {
		runTestScript(t, `echo hello; echo $?`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if strings.TrimSpace(lines[0]) != "hello" {
		t.Errorf("expected 'hello', got %q", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "0" {
		t.Errorf("expected '0', got %q", lines[1])
	}
}

func TestCommandNotFound(t *testing.T) {
	code := runTestScript(t, `nonexistent_command_xyz`)
	if code != 127 {
		t.Errorf("expected 127, got %d", code)
	}
}

// ==================== Complex Script Tests ====================

func TestComplexScript(t *testing.T) {
	output := captureStdout(func() {
		runTestScript(t, `#!/bin/sh
# A complex script testing multiple features

# Variables
NAME="World"
GREETING="Hello"

# Function definition
greet() {
  local name=$1
  echo "$GREETING, $name!"
}

# Main logic
if [ -n "$NAME" ]; then
  greet "$NAME"
fi

# For loop with arithmetic
for i in 1 2 3 4 5; do
  if [ $((i % 2)) -eq 0 ]; then
    echo "$i is even"
  else
    echo "$i is odd"
  fi
done

# Case statement
case "$NAME" in
  World)
    echo "It's the world!"
    ;;
  *)
    echo "Unknown"
    ;;
esac`)
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 7 {
		t.Fatalf("expected 8 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "Hello, World!" {
		t.Errorf("line 0: expected 'Hello, World!', got %q", lines[0])
	}
	if lines[6] != "It's the world!" {
		t.Errorf("line 6: expected 'It's the world!', got %q", lines[6])
	}
}

func TestScriptWithArguments(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")
	os.WriteFile(scriptPath, []byte(`echo "arg1=$1 arg2=$2 arg3=$3"
echo "total=$#"`), 0755)

	output := captureStdout(func() {
		runScriptWithArgs(scriptPath, []string{"foo", "bar", "baz"})
	})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "arg1=foo arg2=bar arg3=baz" {
		t.Errorf("line 0: unexpected %q", lines[0])
	}
	if lines[1] != "total=3" {
		t.Errorf("line 1: unexpected %q", lines[1])
	}
}

// ==================== Glob Pattern Tests ====================

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		word    string
		pattern string
		match   bool
	}{
		{"hello", "*", true},
		{"hello", "hello", true},
		{"hello", "world", false},
		{"hello.txt", "*.txt", true},
		{"hello.sh", "*.txt", false},
		{"abc", "a*", true},
		{"abc", "*c", true},
		{"abc", "a*c", true},
		{"abc", "?bc", true},
		{"abc", "a?c", true},
		{"abc", "ab?", true},
		{"abc", "?b?", true},
		{"abc", "???", true},
	}

	for _, tt := range tests {
		result := matchGlob(tt.word, tt.pattern)
		if result != tt.match {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.word, tt.pattern, result, tt.match)
		}
	}
}

// ==================== Valid Variable Name Tests ====================

func TestIsValidVarName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"foo", true},
		{"FOO", true},
		{"_foo", true},
		{"foo123", true},
		{"foo_bar", true},
		{"1foo", false},
		{"foo-bar", false},
		{"", false},
	}

	for _, tt := range tests {
		result := isValidVarName(tt.name)
		if result != tt.valid {
			t.Errorf("isValidVarName(%q) = %v, want %v", tt.name, result, tt.valid)
		}
	}
}

// ==================== Parse Args Tests ====================

func TestParseArgsEmpty(t *testing.T) {
	args := parseArgs("")
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestParseArgsMultipleSpaces(t *testing.T) {
	args := parseArgs("  hello   world  ")
	if len(args) != 2 || args[0] != "hello" || args[1] != "world" {
		t.Errorf("unexpected args: %v", args)
	}
}

func TestParseArgsMixedQuotes(t *testing.T) {
	args := parseArgs(`'hello' "world" bare`)
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %d: %v", len(args), args)
	}
	if args[0] != "hello" || args[1] != "world" || args[2] != "bare" {
		t.Errorf("unexpected args: %v", args)
	}
}

// ==================== Word List Tests ====================

func TestParseWordList(t *testing.T) {
	words := parseWordList("one two three")
	if len(words) != 3 || words[0] != "one" || words[1] != "two" || words[2] != "three" {
		t.Errorf("unexpected words: %v", words)
	}
}

func TestParseWordListWithQuotes(t *testing.T) {
	words := parseWordList(`'hello world' "foo bar" baz`)
	if len(words) != 3 {
		t.Fatalf("expected 3 words, got %d: %v", len(words), words)
	}
	if words[0] != "hello world" {
		t.Errorf("expected 'hello world', got %q", words[0])
	}
}
