package chatartifact

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

func generateZIP(spec ArtifactSpec) ([]byte, error) {
	type manifestFile struct {
		Filename string `json:"filename"`
		MIME     string `json:"mime"`
		Bytes    int    `json:"bytes"`
	}
	type entry struct {
		name string
		data []byte
		meta manifestFile
	}
	entries := make([]entry, 0, len(spec.ZipFiles))
	for _, child := range spec.ZipFiles {
		data, mime, filename, err := Generate(child)
		if err != nil {
			return nil, err
		}
		if err := validateFilename(filename); err != nil {
			return nil, err
		}
		entries = append(entries, entry{filename, data, manifestFile{filename, mime, len(data)}})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	manifest := struct {
		Title string         `json:"title"`
		Files []manifestFile `json:"files"`
	}{Title: spec.Title}
	for _, item := range entries {
		manifest.Files = append(manifest.Files, item.meta)
	}
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, item := range entries {
		f, err := w.Create(item.name)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %q: %w", item.name, err)
		}
		if _, err := f.Write(item.data); err != nil {
			return nil, err
		}
	}
	f, err := w.Create("manifest.json")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(append(manifestData, '\n')); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
