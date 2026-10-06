// Package debuglog is the manager's runtime debug switch and its in-memory log.
//
// Debug logging is off by default: the manager is quiet unless the operator asks
// for it, from the Settings tab or with DCSMANAGER_DEBUG. When on, a line is kept
// for the notable actions (HTTP requests, panel inputs and the commands they
// produce, DCS-BIOS frames, mission transitions, debrief transfers) and streamed
// to the UI, so the manager can be watched live instead of digging in a file.
package debuglog

import (
	"fmt"
	"sync"
	"time"
)

// Line is one recorded action or message.
type Line struct {
	// At is when it happened, in unix milliseconds (the UI renders it in local
	// time, like the other live frames).
	At int64 `json:"at"`
	// Level is "info", "warn" or "error".
	Level string `json:"level"`
	// Area is the component that logged it ("http", "panel", "dcsbios", "mapping",
	// "mission", "debrief", "app"…), so the UI can colour or filter by source.
	Area string `json:"area"`
	// Message is the human sentence.
	Message string `json:"message"`
}

// maxLines bounds the ring buffer. It is the UI log, not the file: a few hundred
// lines are enough to follow what is happening, and the file keeps everything.
const maxLines = 500

// Logger records debug lines and forwards each one to a sink (the SSE broadcaster).
type Logger struct {
	mu    sync.Mutex
	on    bool
	lines []Line
	// sink is called for every recorded line while debug is on. It must not block.
	sink func(Line)
}

// New creates a logger. on is the initial state; sink may be nil.
func New(on bool, sink func(Line)) *Logger {
	return &Logger{on: on, sink: sink}
}

// SetSink replaces the destination of live lines. It is called once the API is
// ready to broadcast.
func (l *Logger) SetSink(sink func(Line)) {
	l.mu.Lock()
	l.sink = sink
	l.mu.Unlock()
}

// Enabled reports whether debug logging is on.
func (l *Logger) Enabled() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.on
}

// SetEnabled turns debug logging on or off. Turning it on does not clear what was
// already recorded.
func (l *Logger) SetEnabled(on bool) {
	l.mu.Lock()
	l.on = on
	l.mu.Unlock()
}

// Log records one line when debug is on, and does nothing otherwise. It is the
// call sites' single entry point: they do not check Enabled themselves.
func (l *Logger) Log(level, area, msg string) {
	l.mu.Lock()
	if !l.on {
		l.mu.Unlock()
		return
	}
	line := Line{At: time.Now().UnixMilli(), Level: level, Area: area, Message: msg}
	l.lines = append(l.lines, line)
	if len(l.lines) > maxLines {
		// Drop the oldest: the file keeps the full history.
		l.lines = l.lines[len(l.lines)-maxLines:]
	}
	sink := l.sink
	l.mu.Unlock()

	if sink != nil {
		sink(line)
	}
}

// Infof, Warnf and Errorf are the formatted shorthands used across the manager.
func (l *Logger) Infof(area, format string, args ...any) {
	l.Log("info", area, fmt.Sprintf(format, args...))
}

func (l *Logger) Warnf(area, format string, args ...any) {
	l.Log("warn", area, fmt.Sprintf(format, args...))
}

func (l *Logger) Errorf(area, format string, args ...any) {
	l.Log("error", area, fmt.Sprintf(format, args...))
}

// Lines returns a copy of the recorded lines, newest first, so the UI can show
// the most recent action on top.
func (l *Logger) Lines() []Line {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]Line, len(l.lines))
	for i, line := range l.lines {
		out[len(l.lines)-1-i] = line
	}
	return out
}

// Clear empties the in-memory log.
func (l *Logger) Clear() {
	l.mu.Lock()
	l.lines = nil
	l.mu.Unlock()
}
