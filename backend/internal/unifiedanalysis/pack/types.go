package pack

// GeneratedFile is one in-memory pack file (JSON doc or CSV table) before ZIP assembly.
type GeneratedFile struct {
	Name     string
	Content  []byte
	SHA256   string // hex-encoded
	RowCount int    // CSV data rows excluding header; 0 for JSON files
	Kind     string // "json" | "csv"
}

// csvTable is the intermediate representation of one CSV file: a fixed header
// plus deterministically ordered string rows.
type csvTable struct {
	Name    string
	Columns []string
	Rows    [][]string
}
