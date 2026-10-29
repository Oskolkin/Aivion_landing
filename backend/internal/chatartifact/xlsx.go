package chatartifact

import (
	"bytes"
	"fmt"

	"github.com/xuri/excelize/v2"
)

func generateXLSX(spec ArtifactSpec) ([]byte, error) {
	f := excelize.NewFile()
	if err := f.SetDocProps(&excelize.DocProperties{
		Title: spec.Title, Creator: "chatartifact", LastModifiedBy: "chatartifact",
		Created: "1970-01-01T00:00:00Z", Modified: "1970-01-01T00:00:00Z",
	}); err != nil {
		return nil, err
	}
	sheets := []ArtifactSheet{{Name: "Report"}}
	if spec.Workbook != nil && len(spec.Workbook.Sheets) > 0 {
		sheets = spec.Workbook.Sheets
	} else if len(spec.Tables) > 0 {
		sheets[0].Columns = spec.Tables[0].Columns
		sheets[0].Rows = spec.Tables[0].Rows
	}
	for i, sheet := range sheets {
		name := sheet.Name
		if name == "" {
			name = fmt.Sprintf("Sheet%d", i+1)
		}
		if i == 0 {
			f.SetSheetName("Sheet1", name)
		} else if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
		for col, value := range sheet.Columns {
			cell, _ := excelize.CoordinatesToCellName(col+1, 1)
			if err := f.SetCellValue(name, cell, escapeFormulaCell(value)); err != nil {
				return nil, err
			}
		}
		for rowIndex, row := range sheet.Rows {
			for col, value := range row {
				cell, _ := excelize.CoordinatesToCellName(col+1, rowIndex+2)
				if err := f.SetCellValue(name, cell, escapeFormulaCell(value)); err != nil {
					return nil, err
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
