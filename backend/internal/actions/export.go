package actions

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

var exportSafeChars = regexp.MustCompile(`[^a-z0-9._-]+`)
var formulaPrefixPattern = regexp.MustCompile(`^[=+\-@]`)

func supportsActionExport(actionType ActionType) bool {
	switch actionType {
	case ActionTypeStockReplenishment,
		ActionTypeStockRedistribution,
		ActionTypeSlowTurnoverReview,
		ActionTypeNoSalesWithStock,
		ActionTypeSKUOrdersDrop,
		ActionTypeKeySKURisk,
		ActionTypeKeySKUProtection,
		ActionTypeAdHighDRR,
		ActionTypeAdHighDRRReview,
		ActionTypeAdSpendNoOrders,
		ActionTypeAdBudgetReallocation,
		ActionTypeFunnelWeakTransition,
		ActionTypeCardConversionReview,
		ActionTypeCategoryUnderperform,
		ActionTypeSalesDropReview,
		ActionTypeDataQualityIssue:
		return true
	default:
		return false
	}
}

func safeExportFilename(item ActionItem, format string) string {
	datePart := item.MetricDate
	if strings.TrimSpace(datePart) == "" {
		datePart = time.Now().UTC().Format("2006-01-02")
	}
	stem := fmt.Sprintf("aivion-action-%s-%s", item.Type, datePart)
	stem = strings.ToLower(stem)
	stem = exportSafeChars.ReplaceAllString(stem, "-")
	stem = strings.Trim(stem, "-.")
	if stem == "" {
		stem = "aivion-action-export"
	}
	ext := ".xlsx"
	if format == "csv" {
		ext = ".csv"
	}
	return stem + ext
}

func renderActionExportCSV(item ActionItem, entityCtx *ExportEntityContext) ([]byte, string, error) {
	var buf bytes.Buffer
	buf.Write([]byte{0xEF, 0xBB, 0xBF})
	w := csv.NewWriter(&buf)
	w.Comma = ';'
	headers := []string{
		"action_id", "action_key", "run_id", "pipeline_run_id", "title", "type", "category", "priority", "status", "metric_date",
		"summary", "reason", "next_step", "expected_effect", "completion_criteria", "limitations", "visible_limitations",
		"entity_group_ref", "evidence_fact_ids",
		"evidence_kind", "evidence_label", "evidence_value", "evidence_unit", "evidence_description",
		"object_type", "object_id", "object_name", "object_source", "export_warning",
	}
	if err := w.Write(headers); err != nil {
		return nil, "", err
	}
	warning := ""
	if entityCtx != nil {
		warning = entityCtx.Warning
	}
	objectRows := buildExportObjectRows(item, entityCtx)
	base := buildActionExportBaseRow(item)
	if len(item.Evidence) == 0 && len(objectRows) == 0 {
		row := append(append([]string(nil), base...), "", "", "", "", "", "", "", "", sanitizeExportCell(warning))
		if err := w.Write(row); err != nil {
			return nil, "", err
		}
	} else if len(item.Evidence) == 0 {
		for _, obj := range objectRows {
			row := append(append([]string(nil), base...), "", "", "", "", "")
			row = append(row,
				obj.ObjectType,
				sanitizeExportCell(obj.ObjectID),
				sanitizeExportCell(obj.Name),
				obj.Source,
				sanitizeExportCell(warning),
			)
			if err := w.Write(row); err != nil {
				return nil, "", err
			}
		}
	} else {
		for i, ev := range item.Evidence {
			row := append(append([]string(nil), base...),
				ev.Kind, sanitizeExportCell(ev.Label), sanitizeExportCell(ev.Value), ev.Unit, sanitizeExportCell(ev.Description))
			if i < len(objectRows) {
				obj := objectRows[i]
				row = append(row,
					obj.ObjectType,
					sanitizeExportCell(obj.ObjectID),
					sanitizeExportCell(obj.Name),
					obj.Source,
					sanitizeExportCell(warning),
				)
			} else {
				row = append(row, "", "", "", "", sanitizeExportCell(warning))
			}
			if err := w.Write(row); err != nil {
				return nil, "", err
			}
		}
		if len(objectRows) > len(item.Evidence) {
			for _, obj := range objectRows[len(item.Evidence):] {
				row := append(append([]string(nil), base...), "", "", "", "", "")
				row = append(row,
					obj.ObjectType,
					sanitizeExportCell(obj.ObjectID),
					sanitizeExportCell(obj.Name),
					obj.Source,
					sanitizeExportCell(warning),
				)
				if err := w.Write(row); err != nil {
					return nil, "", err
				}
			}
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "text/csv; charset=utf-8", nil
}

func renderActionExportXLSX(item ActionItem, entityCtx *ExportEntityContext) ([]byte, string, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	mainSheet := "Действие"
	f.SetSheetName("Sheet1", mainSheet)
	actionRows := [][]string{
		{"ID", fmt.Sprintf("%d", item.ID)},
		{"Run ID", fmt.Sprintf("%d", item.RunID)},
		{"Название", item.Title},
		{"Тип", string(item.Type)},
		{"Категория", string(item.Category)},
		{"Приоритет", string(item.Priority)},
		{"Статус", string(item.Status)},
		{"Дата анализа", item.MetricDate},
		{"Краткий вывод", item.Summary},
		{"Почему", item.Reason},
		{"Что сделать", item.NextStep},
		{"Ожидаемый эффект", item.ExpectedEffect},
		{"Критерий выполнения", item.CompletionCriteria},
		{"Ограничения анализа", strings.Join(item.VisibleLimitations, " | ")},
		{"Legacy limitations", strings.Join(item.Limitations, " | ")},
		{"Entity group ref", item.EntityGroupRef},
	}
	if item.Source != nil {
		actionRows = append(actionRows,
			[]string{"Pipeline run ID", fmt.Sprintf("%d", item.Source.PipelineRunID)},
			[]string{"Actions run ID", fmt.Sprintf("%d", item.Source.ActionsRunID)},
			[]string{"Data pack hash", item.Source.DataPackHash},
			[]string{"Global context hash", item.Source.GlobalContextHash},
		)
	}
	if entityCtx != nil && entityCtx.Warning != "" {
		actionRows = append(actionRows, []string{"Предупреждение", entityCtx.Warning})
	}
	for i, row := range actionRows {
		cellA := fmt.Sprintf("A%d", i+1)
		cellB := fmt.Sprintf("B%d", i+1)
		_ = f.SetCellValue(mainSheet, cellA, row[0])
		_ = f.SetCellValue(mainSheet, cellB, sanitizeExportCell(row[1]))
	}

	evidenceSheet := "Доказательства"
	_, _ = f.NewSheet(evidenceSheet)
	headers := []string{"Kind", "Label", "Value", "Unit", "Direction", "Description"}
	for i, h := range headers {
		cell := fmt.Sprintf("%c1", 'A'+i)
		_ = f.SetCellValue(evidenceSheet, cell, h)
	}
	for i, ev := range item.Evidence {
		r := i + 2
		_ = f.SetCellValue(evidenceSheet, fmt.Sprintf("A%d", r), ev.Kind)
		_ = f.SetCellValue(evidenceSheet, fmt.Sprintf("B%d", r), sanitizeExportCell(ev.Label))
		_ = f.SetCellValue(evidenceSheet, fmt.Sprintf("C%d", r), sanitizeExportCell(ev.Value))
		_ = f.SetCellValue(evidenceSheet, fmt.Sprintf("D%d", r), ev.Unit)
		_ = f.SetCellValue(evidenceSheet, fmt.Sprintf("E%d", r), ev.Direction)
		_ = f.SetCellValue(evidenceSheet, fmt.Sprintf("F%d", r), sanitizeExportCell(ev.Description))
	}

	objectsSheet := "Объекты"
	_, _ = f.NewSheet(objectsSheet)
	objHeaders := []string{"Object Type", "Object ID", "Name", "Offer ID", "Metric/value", "Source"}
	for i, h := range objHeaders {
		cell := fmt.Sprintf("%c1", 'A'+i)
		_ = f.SetCellValue(objectsSheet, cell, h)
	}
	row := 2
	for _, obj := range buildExportObjectRows(item, entityCtx) {
		_ = f.SetCellValue(objectsSheet, fmt.Sprintf("A%d", row), obj.ObjectType)
		_ = f.SetCellValue(objectsSheet, fmt.Sprintf("B%d", row), sanitizeExportCell(obj.ObjectID))
		_ = f.SetCellValue(objectsSheet, fmt.Sprintf("C%d", row), sanitizeExportCell(obj.Name))
		_ = f.SetCellValue(objectsSheet, fmt.Sprintf("D%d", row), sanitizeExportCell(obj.OfferID))
		_ = f.SetCellValue(objectsSheet, fmt.Sprintf("E%d", row), sanitizeExportCell(obj.Metric))
		_ = f.SetCellValue(objectsSheet, fmt.Sprintf("F%d", row), obj.Source)
		row++
	}

	var raw bytes.Buffer
	if err := f.Write(&raw); err != nil {
		return nil, "", err
	}
	return raw.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", nil
}

func sanitizeExportCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t")
	if formulaPrefixPattern.MatchString(trimmed) {
		return "'" + value
	}
	return value
}

func buildActionExportBaseRow(item ActionItem) []string {
	pipelineRunID := ""
	if item.Source != nil && item.Source.PipelineRunID > 0 {
		pipelineRunID = fmt.Sprintf("%d", item.Source.PipelineRunID)
	}
	return []string{
		fmt.Sprintf("%d", item.ID),
		sanitizeExportCell(item.ActionKey),
		fmt.Sprintf("%d", item.RunID),
		pipelineRunID,
		sanitizeExportCell(item.Title),
		string(item.Type),
		string(item.Category),
		string(item.Priority),
		string(item.Status),
		item.MetricDate,
		sanitizeExportCell(item.Summary),
		sanitizeExportCell(item.Reason),
		sanitizeExportCell(item.NextStep),
		sanitizeExportCell(item.ExpectedEffect),
		sanitizeExportCell(item.CompletionCriteria),
		sanitizeExportCell(strings.Join(item.Limitations, " | ")),
		sanitizeExportCell(strings.Join(item.VisibleLimitations, " | ")),
		sanitizeExportCell(item.EntityGroupRef),
		sanitizeExportCell(strings.Join(item.EvidenceFactIDs, " | ")),
	}
}
