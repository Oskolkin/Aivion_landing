package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dashboardhome"
)

// We cannot call unexported funcs from another package. Use HTTP against live API after restart instead.
func main() {
	raw, err := os.ReadFile("debug/_wc_586_utf8.json")
	if err != nil {
		panic(err)
	}
	var wc map[string]any
	if err := json.Unmarshal(raw, &wc); err != nil {
		panic(err)
	}
	fmt.Println("groups", len(wc["groups"].([]any)))
}
