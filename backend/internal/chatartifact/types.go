// Package chatartifact creates deterministic downloadable chat artifacts.
package chatartifact

type ArtifactSpec struct {
	Type     string
	Filename string
	Title    string
	Sections []ArtifactSection
	Tables   []ArtifactTable
	Workbook *ArtifactWorkbook
	Slides   []ArtifactSlide
	Text     string
	JSON     any
	Chart    *ArtifactChart
	ZipFiles []ArtifactSpec
}

type ArtifactSection struct {
	Heading    string
	Paragraphs []string
	Bullets    []string
}

type ArtifactTable struct {
	Columns []string
	Rows    [][]string
}

type ArtifactWorkbook struct {
	Sheets []ArtifactSheet
}

type ArtifactSheet struct {
	Name    string
	Columns []string
	Rows    [][]string
}

type ArtifactSlide struct {
	Title   string
	Body    string
	Bullets []string
}

type ArtifactChart struct {
	Title  string
	Labels []string
	Values []float64
}
