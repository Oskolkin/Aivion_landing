//go:build integration

package analysis_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/auth"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chat"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/chatv2"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/dbgen"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factual"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/factualapi"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/httpserver/handlers"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func prompt15CurrentPublicationFixture(t *testing.T) (prompt81Fixture, int64, int64) {
	t.Helper()
	f, r1, _ := prompt15P1P2Stock(t)
	p1 := prompt14PubID(t, f, r1)
	execHist, _ := prompt132Exec(t, f, "p15-export-hist")
	for i := 0; i < 4; i++ {
		r := f.prodRun(t)
		prompt132RunThrough(t, execHist, r.ID, "")
	}
	var currentPub int64
	if err := f.pool.QueryRow(context.Background(), `SELECT publication_id FROM seller_current_publications WHERE seller_account_id=$1`, f.seller).Scan(&currentPub); err != nil {
		t.Fatal(err)
	}
	if currentPub == p1 {
		t.Fatal("expected later current publication after historical advance runs")
	}
	return f, currentPub, prompt15PolarisID(t, f)
}

func prompt15RealMinIOArtifacts(t *testing.T) *chatv2.S3ArtifactStore {
	t.Helper()
	endpoint := strings.TrimSpace(os.Getenv("S3_ENDPOINT"))
	if endpoint == "" {
		endpoint = strings.TrimSpace(os.Getenv("TEST_S3_ENDPOINT"))
	}
	if endpoint == "" && os.Getenv("PROMPT15_E2E_MINIO") != "1" {
		t.Skip("set PROMPT15_E2E_MINIO=1 or S3_ENDPOINT for real MinIO export preflight")
	}
	if endpoint == "" {
		endpoint = "127.0.0.1:19000"
	}
	access := firstNonEmpty(os.Getenv("S3_ACCESS_KEY"), os.Getenv("TEST_S3_ACCESS_KEY"), "minio")
	secret := firstNonEmpty(os.Getenv("S3_SECRET_KEY"), os.Getenv("TEST_S3_SECRET_KEY"), "minio123")
	exports := firstNonEmpty(os.Getenv("S3_BUCKET_EXPORTS"), os.Getenv("TEST_S3_BUCKET_EXPORTS"), "exports")
	raw := firstNonEmpty(os.Getenv("S3_BUCKET_RAW"), os.Getenv("TEST_S3_BUCKET_RAW"), "raw-payloads")
	arts := firstNonEmpty(os.Getenv("S3_BUCKET_ARTIFACTS"), os.Getenv("TEST_S3_BUCKET_ARTIFACTS"), "artifacts")
	client, err := storage.New(context.Background(), storage.S3Config{
		Endpoint: endpoint, AccessKey: access, SecretKey: secret, UseSSL: false,
		BucketRaw: raw, BucketExports: exports, BucketArtifacts: arts,
	})
	if err != nil {
		if os.Getenv("PROMPT15_REQUIRE_MINIO") == "1" {
			t.Fatalf("MinIO required but unavailable: %v", err)
		}
		t.Skipf("MinIO not available: %v", err)
	}
	store := chatv2.NewS3ArtifactStore(client, exports)
	if store == nil {
		t.Fatal("S3ArtifactStore nil")
	}
	return store
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type memInputStorage struct {
	data map[string][]byte
}

func (m *memInputStorage) UploadInputAttachment(_ context.Context, bucket, objectKey string, data []byte, _ string) error {
	if m.data == nil {
		m.data = map[string][]byte{}
	}
	cp := append([]byte(nil), data...)
	m.data[bucket+"/"+objectKey] = cp
	return nil
}

func (m *memInputStorage) DownloadInputAttachment(_ context.Context, bucket, objectKey string) ([]byte, error) {
	data, ok := m.data[bucket+"/"+objectKey]
	if !ok {
		return nil, errors.New("input object not found")
	}
	return append([]byte(nil), data...), nil
}

func (m *memInputStorage) DeleteInputAttachment(_ context.Context, bucket, objectKey string) error {
	delete(m.data, bucket+"/"+objectKey)
	return nil
}

func TestPrompt15CurrentPublicationExportDatasetPreflight(t *testing.T) {
	f, currentPub, skuID := prompt15CurrentPublicationFixture(t)
	ctx := context.Background()
	svc := prompt15ChatService(t, f, chatv2.NewFakeProvider())
	sess, err := svc.Store.CreateSession(ctx, f.seller, nil, "export-preflight")
	if err != nil {
		t.Fatal(err)
	}
	userMsgID, err := svc.Store.CreateMessage(ctx, sess.ID, f.seller, "user", "Выгрузи продажи по Polaris 417", "question")
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := svc.Resolver.Resolve(ctx, f.seller, sess.ID, "Выгрузи продажи по Polaris 417", nil, nil, userMsgID)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.ContextMode != chatv2.ContextPublicationBound {
		t.Fatalf("context_mode=%s", frozen.ContextMode)
	}
	if frozen.Publication == nil || frozen.Publication.ID != currentPub {
		t.Fatalf("publication=%v want %d", frozen.Publication, currentPub)
	}
	if frozen.Publication.UniverseID == "" || frozen.Publication.CutoffAt.IsZero() || frozen.FactualIdentity == "" {
		t.Fatalf("incomplete frozen publication %#v identity=%q", frozen.Publication, frozen.FactualIdentity)
	}

	secret := strings.Repeat("t", 32)
	signer, err := factualapi.NewTokenSigner([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	api := factual.NewService(f.pool, signer, 0, 0)
	found, err := api.SearchEntities(ctx, frozen.Factual, factualapi.EntitySearchRequest{
		Query: "Выгрузи продажи по Polaris 417",
		Kinds: []factualapi.EntitySearchKind{factualapi.EntitySearchSKU},
		Limit: 8,
	})
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, it := range found.Items {
		if it.CanonicalID == skuID {
			ok = true
		}
	}
	if !ok {
		t.Fatalf("search missed Polaris sku=%d items=%+v", skuID, found.Items)
	}

	spec := &chatv2.ExportDatasetRef{
		Kind: "SKU", Period: "last_7d",
		Filters: []chatv2.ExportFilterClause{{Field: "sku_id", Operator: "eq", Value: strconv.FormatInt(skuID, 10)}},
	}
	start := time.Now()
	resolved, err := (&chatv2.DatasetResolver{API: api}).Resolve(ctx, frozen, spec)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("DatasetResolver.Resolve failed: %v", err)
	}
	if len(resolved.Rows) == 0 || resolved.PeriodFrom.IsZero() || resolved.PeriodTo.IsZero() {
		t.Fatalf("invalid resolved dataset %#v", resolved)
	}
	csv := strings.Join(flattenRows(resolved.Rows), "\n")
	for _, metric := range []string{"ordered_units", "ordered_amount", "sellable_stock"} {
		if !strings.Contains(csv, metric) {
			t.Fatalf("missing metric %s in %s", metric, csv)
		}
	}
	t.Logf("current_pub=%d universe=%s sku=%d rows=%d period=%s..%s resolve=%s",
		currentPub, frozen.Publication.UniverseID, skuID, len(resolved.Rows),
		resolved.PeriodFrom.Format("2006-01-02"), resolved.PeriodTo.Format("2006-01-02"), elapsed)
}

func flattenRows(rows [][]string) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, strings.Join(r, ","))
	}
	return out
}

func TestPrompt15CurrentPublicationRealMinIOPrepare(t *testing.T) {
	arts := prompt15RealMinIOArtifacts(t)
	f, currentPub, skuID := prompt15CurrentPublicationFixture(t)
	ctx := context.Background()
	svc := prompt15ChatService(t, f, chatv2.NewFakeProvider())
	svc.Exporter.Artifacts = arts
	sess, err := svc.Store.CreateSession(ctx, f.seller, nil, "minio-prepare")
	if err != nil {
		t.Fatal(err)
	}
	userMsgID, err := svc.Store.CreateMessage(ctx, sess.ID, f.seller, "user", "Выгрузи продажи по Polaris 417", "question")
	if err != nil {
		t.Fatal(err)
	}
	frozen, err := svc.Resolver.Resolve(ctx, f.seller, sess.ID, "Выгрузи продажи по Polaris 417", nil, nil, userMsgID)
	if err != nil {
		t.Fatal(err)
	}
	if frozen.Publication == nil || frozen.Publication.ID != currentPub {
		t.Fatalf("publication=%v want %d", frozen.Publication, currentPub)
	}
	result := chatv2.TurnResult{
		Answer: "Файл выгрузки подготовлен.", AnswerMode: chatv2.ModeExport,
		ExportDataset: &chatv2.ExportDatasetRef{
			Kind: "SKU", Period: "last_7d",
			Filters: []chatv2.ExportFilterClause{{Field: "sku_id", Operator: "eq", Value: strconv.FormatInt(skuID, 10)}},
		},
	}
	putStart := time.Now()
	prepared, err := svc.Exporter.Prepare(ctx, f.seller, sess.ID, frozen, result)
	putElapsed := time.Since(putStart)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}
	if prepared == nil || prepared.StorageKey == "" || prepared.Size <= 0 || prepared.ContentHash == "" {
		t.Fatalf("prepared incomplete %#v", prepared)
	}
	if prepared.PeriodFrom.IsZero() || prepared.PeriodTo.IsZero() {
		t.Fatalf("period missing %#v", prepared)
	}
	data, err := arts.Get(ctx, prepared.StorageKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("empty MinIO object")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != prepared.ContentHash {
		t.Fatalf("hash mismatch db=%s actual=%s", prepared.ContentHash, hex.EncodeToString(sum[:]))
	}
	text := string(data)
	for _, needle := range []string{"Polaris", "9105", strconv.FormatInt(skuID, 10), "ordered_units", "ordered_amount", "sellable_stock"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CSV missing %q", needle)
		}
	}
	if err := arts.Delete(ctx, prepared.StorageKey); err != nil {
		t.Fatal(err)
	}
	if _, err := arts.Get(ctx, prepared.StorageKey); err == nil {
		t.Fatal("expected object gone after Delete")
	}
	t.Logf("minio prepare key=%s size=%d put=%s", prepared.StorageKey, prepared.Size, putElapsed)
}

func TestPrompt15DynamicFakeExportStoredDownloadProvenance(t *testing.T) {
	f, currentPub, skuID := prompt15CurrentPublicationFixture(t)
	ctx := context.Background()
	secret := strings.Repeat("t", 32)
	signer, err := factualapi.NewTokenSigner([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	base := factual.NewService(f.pool, signer, 0, 0)
	cnt := &countingFactual{API: base}
	fp := chatv2.NewFakeProvider()
	svc := prompt15ChatServiceAPI(t, f, fp, cnt)
	useMinIO := os.Getenv("PROMPT15_E2E_MINIO") == "1" || strings.TrimSpace(os.Getenv("S3_ENDPOINT")) != "" || strings.TrimSpace(os.Getenv("TEST_S3_ENDPOINT")) != ""
	var s3 *chatv2.S3ArtifactStore
	if useMinIO {
		s3 = prompt15RealMinIOArtifacts(t)
		svc.Exporter.Artifacts = s3
	}
	mem := svc.Exporter.Artifacts
	var events []chatv2.StreamEvent
	start := time.Now()
	res, err := svc.AskStream(ctx, chatv2.AskInput{
		SellerAccountID: f.seller,
		ClientRequestID: "p15-dynamic-export-" + uuid.NewString(),
		Question:        "Выгрузи продажи по Polaris 417",
	}, func(ev chatv2.StreamEvent) error {
		events = append(events, ev)
		return nil
	})
	total := time.Since(start)
	if err != nil {
		t.Fatalf("AskStream failed: %v", err)
	}
	if res == nil || res.ErrorCode != "" || len(res.Attachments) != 1 {
		t.Fatalf("unexpected result %#v", res)
	}
	toolOK := 0
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM chat_turn_tool_calls WHERE turn_id=$1 AND tool_key=$2`, res.TurnID, chatv2.ToolSearchEntities).Scan(&toolOK)
	if toolOK < 1 {
		t.Fatal("expected search_entities tool audit row")
	}
	if cnt.demandGet != 0 || cnt.stockGet != 0 || cnt.skuGet != 0 {
		t.Fatalf("export N+1 demandGet=%d stockGet=%d skuGet=%d", cnt.demandGet, cnt.stockGet, cnt.skuGet)
	}
	if cnt.demandBatch != 1 || cnt.stockBatch != 1 || cnt.skuBatch != 1 {
		t.Fatalf("expected batch=1 demand=%d stock=%d sku=%d", cnt.demandBatch, cnt.stockBatch, cnt.skuBatch)
	}
	attID, err := uuid.Parse(res.Attachments[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	data, row, err := svc.DownloadExport(ctx, f.seller, attID)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 || int64(len(data)) != row.Size {
		t.Fatalf("bytes=%d size=%d", len(data), row.Size)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != row.ContentHash {
		t.Fatal("hash mismatch")
	}
	text := string(data)
	for _, needle := range []string{"Polaris", "9105", strconv.FormatInt(skuID, 10), "ordered_units", "ordered_amount", "sellable_stock"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("CSV missing %q", needle)
		}
	}
	var status, errCode string
	var asst *int64
	var pubID int64
	var uni string
	if err := f.pool.QueryRow(ctx, `SELECT status, COALESCE(error_code,''), assistant_message_id, publication_id, factual_universe_id::text
		FROM chat_turns WHERE id=$1`, res.TurnID).Scan(&status, &errCode, &asst, &pubID, &uni); err != nil {
		t.Fatal(err)
	}
	if status != "COMPLETED" || asst == nil || pubID != currentPub {
		t.Fatalf("turn status=%s asst=%v pub=%d want=%d", status, asst, pubID, currentPub)
	}
	var prov int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM chat_export_provenance WHERE turn_id=$1 AND attachment_id=$2`, res.TurnID, attID).Scan(&prov)
	if prov != 1 {
		t.Fatalf("provenance=%d", prov)
	}
	done, errEvents := 0, 0
	for _, ev := range events {
		if ev.Type == "done" {
			done++
		}
		if ev.Type == "error" {
			errEvents++
		}
	}
	if done != 1 || errEvents != 0 {
		t.Fatalf("terminal done=%d error=%d events=%d", done, errEvents, len(events))
	}
	if s3 != nil {
		got, gerr := s3.Get(ctx, row.StorageKey)
		if gerr != nil || len(got) == 0 {
			t.Fatalf("minio get err=%v bytes=%d", gerr, len(got))
		}
	} else if mem != nil {
		if _, ok := mem.(*chatv2.MemoryArtifactStore); ok {
			// memory path is fine for non-MinIO CI
		}
	}
	t.Logf("dynamic export total=%s demandBatch=%d stockBatch=%d skuBatch=%d bytes=%d", total, cnt.demandBatch, cnt.stockBatch, cnt.skuBatch, len(data))
}

func TestPrompt15ExportFinalizationFailureBecomesDurableFailed(t *testing.T) {
	f, _, skuID := prompt15CurrentPublicationFixture(t)
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{Final: &chatv2.TurnResult{
		Answer: "Файл подготовлен.", AnswerMode: chatv2.ModeExport,
		ExportDataset: &chatv2.ExportDatasetRef{
			Kind: "SKU", Period: "last_7d",
			Filters: []chatv2.ExportFilterClause{{Field: "sku_id", Operator: "eq", Value: strconv.FormatInt(skuID, 10)}},
		},
	}})
	svc := prompt15ChatService(t, f, fp)
	svc.Store.Failpoint = chatv2.FailExportProvenance
	mem := svc.Exporter.Artifacts.(*chatv2.MemoryArtifactStore)
	res, err := svc.Ask(context.Background(), chatv2.AskInput{
		SellerAccountID: f.seller, ClientRequestID: "p15-export-fin-fail", Question: "Выгрузи продажи по Polaris 417",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.ErrorCode != "EXPORT_FINALIZATION_FAILED" {
		t.Fatalf("got %#v", res)
	}
	var status, code, idem string
	var asst *int64
	_ = f.pool.QueryRow(context.Background(), `SELECT status, COALESCE(error_code,''), assistant_message_id FROM chat_turns WHERE id=$1`, res.TurnID).Scan(&status, &code, &asst)
	_ = f.pool.QueryRow(context.Background(), `SELECT status FROM chat_request_idempotency WHERE seller_account_id=$1 AND client_request_id='p15-export-fin-fail'`, f.seller).Scan(&idem)
	var asstCount, attCount, provCount int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_messages WHERE seller_account_id=$1 AND role='assistant'`, f.seller).Scan(&asstCount)
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_attachments WHERE seller_account_id=$1 AND attachment_kind='export'`, f.seller).Scan(&attCount)
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_export_provenance WHERE seller_account_id=$1`, f.seller).Scan(&provCount)
	if status != "FAILED" || code != "EXPORT_FINALIZATION_FAILED" || asst != nil || idem != "failed" {
		t.Fatalf("status=%s code=%s asst=%v idem=%s", status, code, asst, idem)
	}
	if asstCount != 0 || attCount != 0 || provCount != 0 {
		t.Fatalf("asst=%d att=%d prov=%d", asstCount, attCount, provCount)
	}
	if mem.Count() != 0 {
		t.Fatalf("orphan objects=%d", mem.Count())
	}
}

func TestPrompt15ExportFinalizationFailureCleansArtifact(t *testing.T) {
	TestPrompt15ExportFinalizationFailureBecomesDurableFailed(t)
}

func TestPrompt15ExportFinalizationFailpointMatrix(t *testing.T) {
	f, _, skuID := prompt15CurrentPublicationFixture(t)
	fps := []chatv2.FinalizeFailpoint{
		chatv2.FailAfterAssistant,
		chatv2.FailExportMeta,
		chatv2.FailExportProvenance,
		chatv2.FailToolAudit,
		chatv2.FailGrounding,
		chatv2.FailTurnComplete,
		chatv2.FailIdempotency,
		chatv2.FailSessionTouch,
		chatv2.FailCommit,
	}
	for _, fp := range fps {
		fp := fp
		t.Run(string(fp), func(t *testing.T) {
			provider := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{Final: &chatv2.TurnResult{
				Answer: "Файл подготовлен.", AnswerMode: chatv2.ModeExport,
				ExportDataset: &chatv2.ExportDatasetRef{
					Kind: "SKU", Period: "last_7d",
					Filters: []chatv2.ExportFilterClause{{Field: "sku_id", Operator: "eq", Value: strconv.FormatInt(skuID, 10)}},
				},
			}})
			svc := prompt15ChatService(t, f, provider)
			svc.Store.Failpoint = fp
			mem := svc.Exporter.Artifacts.(*chatv2.MemoryArtifactStore)
			reqID := "p15-matrix-" + string(fp) + "-" + uuid.NewString()
			res, err := svc.Ask(context.Background(), chatv2.AskInput{
				SellerAccountID: f.seller, ClientRequestID: reqID, Question: "Выгрузи продажи по Polaris 417",
			})
			if err != nil {
				t.Fatal(err)
			}
			if res == nil || (res.ErrorCode != "EXPORT_FINALIZATION_FAILED" && res.ErrorCode != "FINALIZATION_FAILED") {
				t.Fatalf("res=%#v", res)
			}
			var prov, completed, done, failed int
			_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_export_provenance WHERE turn_id=$1`, res.TurnID).Scan(&prov)
			_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_turns WHERE id=$1 AND status='COMPLETED'`, res.TurnID).Scan(&completed)
			_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_request_idempotency WHERE seller_account_id=$1 AND client_request_id=$2 AND status='completed'`, f.seller, reqID).Scan(&done)
			_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_turns WHERE id=$1 AND status='FAILED'`, res.TurnID).Scan(&failed)
			if completed != 0 || done != 0 || prov != 0 {
				t.Fatalf("completed=%d idemCompleted=%d prov=%d", completed, done, prov)
			}
			if mem.Count() != 0 {
				t.Fatalf("orphans=%d", mem.Count())
			}
			if failed != 1 {
				t.Fatalf("failed=%d", failed)
			}
		})
	}
}

func TestPrompt15StreamFinalizationErrorEmitsErrorEvent(t *testing.T) {
	f, _, skuID := prompt15CurrentPublicationFixture(t)
	fp := chatv2.NewFakeProvider().Script(chatv2.ProviderResponse{Final: &chatv2.TurnResult{
		Answer: "Файл подготовлен.", AnswerMode: chatv2.ModeExport,
		ExportDataset: &chatv2.ExportDatasetRef{
			Kind: "SKU", Period: "last_7d",
			Filters: []chatv2.ExportFilterClause{{Field: "sku_id", Operator: "eq", Value: strconv.FormatInt(skuID, 10)}},
		},
	}})
	svc := prompt15ChatService(t, f, fp)
	svc.Store.Failpoint = chatv2.FailExportMeta
	h := handlers.NewChatHandler(nil, nil).WithV2(svc)
	body, _ := json.Marshal(map[string]any{
		"question": "Выгрузи продажи по Polaris 417", "client_request_id": "p15-sse-fin-fail",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chat/ask/stream", bytes.NewReader(body))
	req = req.WithContext(auth.WithAuthContext(req.Context(), dbgen.User{ID: 1}, &dbgen.SellerAccount{ID: f.seller}))
	rr := httptest.NewRecorder()
	h.AskStream(rr, req)
	raw := rr.Body.String()
	if !strings.Contains(raw, "event: error") {
		t.Fatalf("missing SSE error event: %s", raw)
	}
	if strings.Contains(raw, "event: done") {
		t.Fatalf("unexpected done event: %s", raw)
	}
	if strings.Contains(raw, "finalize.export") || strings.Contains(raw, "ERROR:") {
		t.Fatalf("seller SSE leaked internal diagnostics: %s", raw)
	}
}

func TestPrompt15V2ExportDownloadDoesNotFallbackOnStorageFailure(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	ctx := context.Background()
	store := chatv2.NewStore(f.pool)
	sess, err := store.CreateSession(ctx, f.seller, nil, "dl-fail")
	if err != nil {
		t.Fatal(err)
	}
	msgID, err := store.CreateMessage(ctx, sess.ID, f.seller, "assistant", "file", "answer")
	if err != nil {
		t.Fatal(err)
	}
	attID := uuid.New()
	key := fmt.Sprintf("chat-exports/%d/%d/%s.csv", f.seller, sess.ID, attID)
	if err := store.InsertExportAttachment(ctx, attID, f.seller, sess.ID, msgID, "chat-export.csv", "text/csv", 12, key, "abc"); err != nil {
		t.Fatal(err)
	}
	svc := &chatv2.Service{
		Store: store,
		Exporter: &chatv2.FactualExporter{
			Store:     store,
			Artifacts: chatv2.FailArtifactStore{GetErr: errors.New("minio get failed")},
		},
	}
	h := handlers.NewChatHandler(nil, nil).WithV2(svc)
	r := chi.NewRouter()
	r.Get("/{id}", h.DownloadAttachment)
	req := httptest.NewRequest(http.MethodGet, "/"+attID.String(), nil)
	req = req.WithContext(auth.WithAuthContext(req.Context(), dbgen.User{ID: 1}, &dbgen.SellerAccount{ID: f.seller}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "failed to download export") {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestPrompt15LegacyInputAttachmentStillFallsBack(t *testing.T) {
	f, _ := prompt14FirstPublication(t)
	ctx := context.Background()
	store := chatv2.NewStore(f.pool)
	sess, err := store.CreateSession(ctx, f.seller, nil, "legacy-input")
	if err != nil {
		t.Fatal(err)
	}
	attID := uuid.New()
	key := "input/" + attID.String()
	payload := []byte("abc")
	mem := &memInputStorage{data: map[string][]byte{"chat-inputs/" + key: payload}}
	if _, err := f.pool.Exec(ctx, `INSERT INTO chat_attachments (
		id, seller_account_id, chat_session_id, file_name, mime_type, format, file_size, storage_key, status, attachment_kind, extraction_status)
		VALUES ($1,$2,$3,'note.txt','text/plain','txt',$4,$5,'uploaded','input','pending')`,
		attID, f.seller, sess.ID, len(payload), key); err != nil {
		t.Fatal(err)
	}
	legacy := chat.NewDownloadOnlyService(
		chat.NewSQLCAttachmentRepository(dbgen.New(f.pool)),
		mem,
		"chat-inputs",
		nil,
	)
	v2 := &chatv2.Service{
		Store: store,
		Exporter: &chatv2.FactualExporter{
			Store:     store,
			Artifacts: chatv2.NewMemoryArtifactStore(),
		},
	}
	h := handlers.NewChatHandler(legacy, nil).WithV2(v2)
	r := chi.NewRouter()
	r.Get("/{id}", h.DownloadAttachment)
	req := httptest.NewRequest(http.MethodGet, "/"+attID.String(), nil)
	req = req.WithContext(auth.WithAuthContext(req.Context(), dbgen.User{ID: 1}, &dbgen.SellerAccount{ID: f.seller}))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	got, _ := io.ReadAll(rr.Body)
	if string(got) != string(payload) {
		t.Fatalf("bytes=%q", got)
	}
}
