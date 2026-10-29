package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/unifiedanalysis/outputvalidator"
	"io"
	"os"
)

func main() {
	packBytes, _ := os.ReadFile("debug/_pulse_pack_rezip.zip")
	idx, err := outputvalidator.BuildPackReferenceIndex(packBytes)
	fmt.Printf("err=%v idx=%v\n", err, idx != nil)
	// manually parse csv from zip
	zr, _ := zip.NewReader(bytes.NewReader(packBytes), int64(len(packBytes)))
	for _, f := range zr.File {
		if f.Name != "account_summary.csv" {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		fmt.Printf("csv bytes=%d head=%q\n", len(b), string(b[:min(120, len(b))]))
		r := csv.NewReader(bytes.NewReader(b))
		h, e1 := r.Read()
		row, e2 := r.Read()
		fmt.Printf("e1=%v e2=%v cols=%d\n", e1, e2, len(h))
		for i, name := range h {
			if name == "revenue_7d" || name == "orders_7d" {
				fmt.Printf("col %s idx=%d val=%s\n", name, i, row[i])
			}
		}
	}
	// reflection-free: Lookup
	f, ok := idx.LookupAccountMetric("account:revenue_7d:current_7d", "revenue_7d", "current_7d")
	fmt.Printf("lookup=%v val=%v\n", ok, f)
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
