package chatv2

import (
	"context"
	"fmt"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatartifact"
	"github.com/google/uuid"
)

type FactualExporter struct {
	Store     *Store
	Artifacts ArtifactStore
	Resolver  *DatasetResolver
	Path      func(id string) string
}

type PreparedExport struct {
	ID            uuid.UUID
	FileName      string
	MimeType      string
	Size          int64
	StorageKey    string
	ContentHash   string
	PeriodFrom    time.Time
	PeriodTo      time.Time
	Spec          ExportDatasetRef
	AttachmentIDs []string
	PubID         *int64
	UniverseID    string
	CtxHash       string
	DTO           AttachmentDTO
}

func (e *FactualExporter) Prepare(ctx context.Context, sellerID, sessionID int64, frozen FrozenTurnContext, result TurnResult) (*PreparedExport, error) {
	if e == nil || e.Resolver == nil || e.Artifacts == nil {
		return nil, ErrExportFailed
	}
	resolved, err := e.Resolver.Resolve(ctx, frozen, result.ExportDataset)
	if err != nil {
		return nil, fmt.Errorf("export dataset resolve: %w", err)
	}
	spec := chatartifact.ArtifactSpec{
		Type: "csv", Filename: "chat-export.csv", Title: "Chat export",
		Tables: []chatartifact.ArtifactTable{{Columns: exportColumns, Rows: resolved.Rows}},
	}
	data, mime, _, err := chatartifact.Generate(spec)
	if err != nil {
		return nil, fmt.Errorf("export csv generate: %w", err)
	}
	hash := hashBytes(data)
	id := uuid.New()
	key := fmt.Sprintf("chat-exports/%d/%d/%s.csv", sellerID, sessionID, id.String())
	if err := e.Artifacts.Put(ctx, key, data, mime); err != nil {
		return nil, fmt.Errorf("export storage put: %w", err)
	}
	name := fmt.Sprintf("chat-export-%s.csv", time.Now().UTC().Format("20060102"))
	url := "/api/v1/chat/attachments/" + id.String() + "/download"
	if e.Path != nil {
		url = e.Path(id.String())
	}
	var pub *int64
	uni := ""
	if frozen.Publication != nil {
		p := frozen.Publication.ID
		pub = &p
		uni = frozen.Publication.UniverseID
	}
	var attIDs []string
	for _, a := range frozen.Attachments {
		attIDs = append(attIDs, a.ID)
	}
	return &PreparedExport{
		ID: id, FileName: name, MimeType: mime, Size: int64(len(data)), StorageKey: key, ContentHash: hash,
		PeriodFrom: resolved.PeriodFrom, PeriodTo: resolved.PeriodTo, Spec: resolved.CanonicalSpec,
		AttachmentIDs: attIDs, PubID: pub, UniverseID: uni, CtxHash: frozen.FactualIdentity,
		DTO: AttachmentDTO{
			ID: id.String(), FileName: name, MimeType: mime, FileSize: int64(len(data)),
			ExportType: "factual_query", Format: "csv", DownloadURL: url, CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func cleanupPreparedExport(prepared *PreparedExport, artifacts ArtifactStore) error {
	if prepared == nil || artifacts == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return artifacts.Delete(ctx, prepared.StorageKey)
}

func (e *FactualExporter) Download(ctx context.Context, sellerID int64, attachmentID uuid.UUID) ([]byte, ExportAttachmentRow, error) {
	if e == nil || e.Store == nil || e.Artifacts == nil {
		return nil, ExportAttachmentRow{}, ErrExportFailed
	}
	row, err := e.Store.GetExportAttachment(ctx, sellerID, attachmentID)
	if err != nil {
		return nil, ExportAttachmentRow{}, fmt.Errorf("export attachment lookup: %w", err)
	}
	data, err := e.Artifacts.Get(ctx, row.StorageKey)
	if err != nil {
		return nil, row, fmt.Errorf("export storage get: %w", err)
	}
	if len(data) == 0 {
		return nil, row, fmt.Errorf("export storage get: empty artifact")
	}
	return data, row, nil
}
