package debuglog

import "testing"

// TestOffByDefault records nothing until it is turned on, which is the whole
// point of the switch: the manager stays quiet unless asked.
func TestOffByDefault(t *testing.T) {
	var got []Line
	l := New(false, func(line Line) { got = append(got, line) })

	l.Infof("http", "GET /api/health")
	if len(got) != 0 || len(l.Lines()) != 0 {
		t.Fatalf("a disabled logger must record nothing: %v / %v", got, l.Lines())
	}

	l.SetEnabled(true)
	l.Infof("http", "GET /api/health")
	if len(got) != 1 || len(l.Lines()) != 1 {
		t.Fatalf("an enabled logger must record: %v / %v", got, l.Lines())
	}
	if got[0].Level != "info" || got[0].Area != "http" {
		t.Errorf("unexpected line: %+v", got[0])
	}
}

// TestLinesNewestFirstAndBounded checks the log is returned newest first and never
// grows past its cap, so the UI shows the most recent action without unbounded
// memory.
func TestLinesNewestFirstAndBounded(t *testing.T) {
	l := New(true, nil)
	for i := 0; i < maxLines+10; i++ {
		l.Infof("app", "line %d", i)
	}

	lines := l.Lines()
	if len(lines) != maxLines {
		t.Fatalf("kept %d lines, want %d", len(lines), maxLines)
	}
	// Newest first: the last logged value is at the front.
	if lines[0].Message != "line "+(itoa(maxLines+9)) {
		t.Errorf("first line = %q, want the newest", lines[0].Message)
	}
	if lines[len(lines)-1].Message != "line 10" {
		t.Errorf("last line = %q, want the oldest kept", lines[len(lines)-1].Message)
	}
}

// TestClear empties the log.
func TestClear(t *testing.T) {
	l := New(true, nil)
	l.Warnf("panel", "something")
	l.Clear()
	if len(l.Lines()) != 0 {
		t.Fatal("Clear should empty the log")
	}
}

// TestSinkReceivesLines checks the live sink is called for each recorded line,
// which is what the SSE stream uses.
func TestSinkReceivesLines(t *testing.T) {
	var count int
	l := New(true, func(Line) { count++ })
	l.Errorf("app", "boom")
	l.Infof("app", "ok")
	if count != 2 {
		t.Fatalf("sink called %d times, want 2", count)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
