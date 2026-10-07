// panelprofiles exports the built-in panel profiles for review or import.
package main

import (
	"dcsmanager/internal/mapping"
	"encoding/json"
	"os"
)

func main() {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(mapping.Starter()); err != nil {
		panic(err)
	}
}
