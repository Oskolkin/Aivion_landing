package chatartifact

import (
	"fmt"
	"strings"
)

func Generate(spec ArtifactSpec) ([]byte, string, string, error) {
	spec.Type = strings.ToLower(strings.TrimSpace(spec.Type))
	if err := ValidateSpec(spec); err != nil {
		return nil, "", "", err
	}
	filename := spec.Filename
	if filename == "" {
		filename = "artifact." + spec.Type
	}
	if !strings.HasSuffix(strings.ToLower(filename), "."+spec.Type) {
		filename += "." + spec.Type
	}
	var data []byte
	var err error
	switch spec.Type {
	case "pdf":
		data, err = generatePDF(spec)
	case "docx":
		data, err = generateDOCX(spec)
	case "xlsx":
		data, err = generateXLSX(spec)
	case "csv", "tsv":
		data, err = generateDelimited(spec, spec.Type == "tsv")
	case "txt", "md", "json":
		data, err = generateText(spec)
	case "pptx":
		data, err = generatePPTX(spec)
	case "png", "jpeg":
		data, err = generateImage(spec, spec.Type)
	case "zip":
		data, err = generateZIP(spec)
	default:
		err = fmt.Errorf("unsupported artifact type %q", spec.Type)
	}
	if err != nil {
		return nil, "", "", err
	}
	return data, mimeFor(spec.Type), filename, nil
}

func mimeFor(kind string) string {
	return map[string]string{
		"pdf": "application/pdf", "docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		"csv":  "text/csv; charset=utf-8", "tsv": "text/tab-separated-values; charset=utf-8",
		"txt": "text/plain; charset=utf-8", "md": "text/markdown; charset=utf-8",
		"json": "application/json", "png": "image/png", "jpeg": "image/jpeg", "zip": "application/zip",
	}[kind]
}
