package chatexport

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatartifact"
	"github.com/xuri/excelize/v2"
)

// WriteFile encodes table data as CSV (UTF-8 BOM, semicolon delimiter) or XLSX.
func WriteFile(format string, sheetName string, table ExportTable) ([]byte, string, error) {
	switch format {
	case FormatCSV:
		data, err := writeCSV(table)
		return data, MIMETypeCSV, err
	case FormatXLSX:
		data, err := writeXLSX(sheetName, table)
		return data, MIMETypeXLSX, err
	default:
		spec := artifactSpecForTable(format, sheetName, table)
		data, mime, _, err := chatartifact.Generate(spec)
		return data, mime, err
	}
}

func artifactSpecForTable(format, title string, table ExportTable) chatartifact.ArtifactSpec {
	artifactTable := chatartifact.ArtifactTable{Columns: table.Columns, Rows: table.Rows}
	spec := chatartifact.ArtifactSpec{
		Type:     format,
		Filename: sanitizeArtifactFilename(title, format),
		Title:    title,
		Tables:   []chatartifact.ArtifactTable{artifactTable},
		Slides:   []chatartifact.ArtifactSlide{{Title: title, Body: fmt.Sprintf("%d rows", len(table.Rows))}},
	}
	if format == FormatTXT || format == FormatMD {
		spec.Text = tableAsText(title, table, format == FormatMD)
	}
	if format == FormatJSON {
		spec.JSON = map[string]any{"title": title, "columns": table.Columns, "rows": table.Rows}
	}
	if format == FormatPNG || format == FormatJPEG {
		spec.Chart = chartForTable(title, table)
	}
	if format == FormatZIP {
		spec.ZipFiles = []chatartifact.ArtifactSpec{
			artifactSpecForTable(FormatXLSX, title, table),
			artifactSpecForTable(FormatCSV, title, table),
			artifactSpecForTable(FormatTXT, title, table),
			artifactSpecForTable(FormatJSON, title, table),
		}
	}
	return spec
}

func sanitizeArtifactFilename(title, format string) string {
	name := strings.TrimSpace(title)
	if name == "" {
		name = "export"
	}
	return strings.ReplaceAll(name, " ", "_") + "." + format
}

func tableAsText(title string, table ExportTable, markdown bool) string {
	if markdown {
		var b strings.Builder
		b.WriteString("# " + title + "\n\n| " + strings.Join(table.Columns, " | ") + " |\n|")
		for range table.Columns {
			b.WriteString(" --- |")
		}
		b.WriteByte('\n')
		for _, row := range table.Rows {
			b.WriteString("| " + strings.Join(row, " | ") + " |\n")
		}
		return b.String()
	}
	data, _ := json.Marshal(map[string]any{"title": title, "columns": table.Columns, "rows": table.Rows})
	return string(data) + "\n"
}

func chartForTable(title string, table ExportTable) *chatartifact.ArtifactChart {
	chart := &chatartifact.ArtifactChart{Title: title}
	if len(table.Columns) == 0 {
		return chart
	}
	numericColumn := -1
	for column := range table.Columns {
		for _, row := range table.Rows {
			if column < len(row) {
				if _, err := strconv.ParseFloat(strings.ReplaceAll(row[column], ",", "."), 64); err == nil {
					numericColumn = column
					break
				}
			}
		}
		if numericColumn >= 0 {
			break
		}
	}
	if numericColumn < 0 {
		return chart
	}
	for _, row := range table.Rows {
		if numericColumn >= len(row) {
			continue
		}
		value, err := strconv.ParseFloat(strings.ReplaceAll(row[numericColumn], ",", "."), 64)
		if err != nil {
			continue
		}
		label := strconv.Itoa(len(chart.Labels) + 1)
		if numericColumn != 0 && len(row) > 0 {
			label = row[0]
		}
		chart.Labels = append(chart.Labels, label)
		chart.Values = append(chart.Values, value)
		if len(chart.Values) == 6 {
			break
		}
	}
	return chart
}

func writeCSV(table ExportTable) ([]byte, error) {
	var buf bytes.Buffer
	// UTF-8 BOM for Excel Cyrillic compatibility.
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	if err := w.Write(escapeFormulaRow(table.Columns)); err != nil {
		return nil, err
	}
	for _, row := range table.Rows {
		if err := w.Write(escapeFormulaRow(row)); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeXLSX(sheetName string, table ExportTable) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	name := sanitizeSheetName(sheetName)
	idx, err := f.NewSheet(name)
	if err != nil {
		return nil, err
	}
	_ = f.DeleteSheet("Sheet1")

	for col, header := range table.Columns {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		_ = f.SetCellValue(name, cell, escapeFormulaCell(header))
	}
	for rowIdx, row := range table.Rows {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(name, cell, cellTypedValue(escapeFormulaCell(value)))
		}
	}
	_ = f.SetPanes(name, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	})
	for col := range table.Columns {
		colName, _ := excelize.ColumnNumberToName(col + 1)
		_ = f.SetColWidth(name, colName, colName, 18)
	}
	f.SetActiveSheet(idx)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sanitizeSheetName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "export"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", "?", "_", "*", "_", "[", "_", "]", "_", ":", "_")
	name = replacer.Replace(name)
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

func cellTypedValue(raw string) any {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil
	}
	if strings.Contains(s, ".") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	return s
}

func fileExtension(format string) string {
	return "." + format
}
