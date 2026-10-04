// Command debriefcheck parses a DCS debrief.log with the same parser the backend
// uses, and prints a summary. It exists to tell a parsing failure apart from a
// transport failure when a debrief is missing from the app: run it on
// Saved Games\DCS\Logs\debrief.log and, if it parses, the loss is upstream.
//
// Usage: go run ./cmd/debriefcheck <path-to-debrief.log>
package main

import (
	"fmt"
	"os"

	"dcsmanager/internal/debrief"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: debriefcheck <path-to-debrief.log>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("read:", err)
		os.Exit(1)
	}
	fmt.Printf("file: %d bytes\n", len(data))

	parsed, err := debrief.Parse(data)
	if err != nil {
		fmt.Println("PARSE ERROR:", err)
		os.Exit(1)
	}
	m := parsed.ToModel()
	fmt.Printf("parsed ok: %d events\n", len(m.Events))
	for i, e := range m.Events {
		if i >= 5 {
			break
		}
		fmt.Printf("  event %d: %+v\n", i, e)
	}
}
