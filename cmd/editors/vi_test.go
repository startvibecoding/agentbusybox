package editors

import "testing"

func TestViInsertAndSaveBuffer(t *testing.T) {
	e := &viEditor{lines: []string{"hello"}, curLine: 0, curCol: 5}
	e.mode = viInsert
	e.insertRune('!')
	if got := e.lines[0]; got != "hello!" {
		t.Fatalf("insertRune = %q, want %q", got, "hello!")
	}
	if !e.modified {
		t.Fatalf("insertRune should mark buffer modified")
	}
}

func TestViSearchForward(t *testing.T) {
	e := &viEditor{lines: []string{"alpha", "beta", "gamma"}, curLine: 0, curCol: 0}
	e.search("beta", true)
	if e.curLine != 1 || e.curCol != 0 {
		t.Fatalf("search moved to (%d,%d), want (1,0)", e.curLine, e.curCol)
	}
}

func TestViJoinLine(t *testing.T) {
	e := &viEditor{lines: []string{"hello", "world"}, curLine: 0, curCol: 0}
	e.joinLine(1)
	if len(e.lines) != 1 || e.lines[0] != "hello world" {
		t.Fatalf("joinLine = %#v", e.lines)
	}
}

func TestViDeleteLineAndYank(t *testing.T) {
	e := &viEditor{lines: []string{"one", "two", "three"}, curLine: 1}
	e.deleteLine(1)
	if len(e.yank) != 1 || e.yank[0] != "two" {
		t.Fatalf("deleteLine yank = %#v", e.yank)
	}
	if len(e.lines) != 2 || e.lines[0] != "one" || e.lines[1] != "three" {
		t.Fatalf("deleteLine lines = %#v", e.lines)
	}
}

func TestViReplaceChars(t *testing.T) {
	e := &viEditor{lines: []string{"hello"}, curLine: 0, curCol: 1}
	e.replaceChars(2, 'x')
	if got := e.lines[0]; got != "hxxlo" {
		t.Fatalf("replaceChars = %q", got)
	}
}

func TestViTakeCount(t *testing.T) {
	e := &viEditor{count: 12}
	if got := e.takeCount(1); got != 12 {
		t.Fatalf("takeCount = %d", got)
	}
	if e.count != 0 {
		t.Fatalf("takeCount should clear count")
	}
}

func TestViUndoPop(t *testing.T) {
	e := &viEditor{lines: []string{"hello"}, curLine: 0, curCol: 5, modified: true}
	e.pushUndo()
	e.lines[0] = "world"
	e.curCol = 0
	e.undoPop()
	if got := e.lines[0]; got != "hello" {
		t.Fatalf("undoPop restored %q", got)
	}
	if e.curCol != 5 {
		t.Fatalf("undoPop cursor = %d, want 5", e.curCol)
	}
}

func TestViHandlePendingDelete(t *testing.T) {
	e := &viEditor{lines: []string{"one", "two", "three"}, curLine: 1}
	e.pending = 'd'
	e.handlePending('d')
	if len(e.lines) != 2 || e.lines[0] != "one" || e.lines[1] != "three" {
		t.Fatalf("pending delete = %#v", e.lines)
	}
	e.undoPop()
	if len(e.lines) != 3 || e.lines[1] != "two" {
		t.Fatalf("undo after delete = %#v", e.lines)
	}
}
