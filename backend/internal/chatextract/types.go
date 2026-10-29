// Package chatextract converts common chat attachments into bounded AI context.
package chatextract

// ExtractedAttachment is normalized content extracted from one attachment.
type ExtractedAttachment struct {
	Filename  string              `json:"filename"`
	Format    string              `json:"format"`
	PlainText string              `json:"plain_text"`
	Sections  []ExtractedSection  `json:"sections,omitempty"`
	Tables    []ExtractedTable    `json:"tables,omitempty"`
	Sheets    []ExtractedSheet    `json:"sheets,omitempty"`
	Image     *ExtractedImageMeta `json:"image,omitempty"`
	Metadata  map[string]any      `json:"metadata,omitempty"`
}

// ExtractedSection is a named content segment, such as a PDF page or slide.
type ExtractedSection struct {
	Title string `json:"title,omitempty"`
	Text  string `json:"text"`
}

// ExtractedTable represents a tabular source.
type ExtractedTable struct {
	Name string     `json:"name,omitempty"`
	Rows [][]string `json:"rows"`
}

// ExtractedSheet is a spreadsheet worksheet.
type ExtractedSheet struct {
	Name  string          `json:"name"`
	Cells []ExtractedCell `json:"cells"`
}

// ExtractedCell retains the A1 address, computed value, and formula.
type ExtractedCell struct {
	Address string `json:"address"`
	Value   string `json:"value,omitempty"`
	Formula string `json:"formula,omitempty"`
}

// ExtractedImageMeta describes a decoded raster image without attempting OCR.
type ExtractedImageMeta struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Format string `json:"format"`
}
