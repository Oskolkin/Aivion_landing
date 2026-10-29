package chatextract

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

func extractXLSX(data []byte) (*ExtractedAttachment, error) {
	book, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = book.Close() }()
	result := &ExtractedAttachment{}
	var plain strings.Builder
	for _, name := range book.GetSheetList() {
		rows, err := book.GetRows(name)
		if err != nil {
			return nil, err
		}
		sheet := ExtractedSheet{Name: name}
		for rowIndex, row := range rows {
			for columnIndex := range row {
				address, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
				if err != nil {
					return nil, err
				}
				value, err := book.GetCellValue(name, address)
				if err != nil {
					return nil, err
				}
				formula, err := book.GetCellFormula(name, address)
				if err != nil {
					return nil, err
				}
				if value == "" && formula == "" {
					continue
				}
				sheet.Cells = append(sheet.Cells, ExtractedCell{Address: address, Value: value, Formula: formula})
				fmt.Fprintf(&plain, "[%s!%s] %s\n", name, address, value)
			}
		}
		result.Sheets = append(result.Sheets, sheet)
	}
	result.PlainText = strings.TrimSpace(plain.String())
	return result, nil
}
