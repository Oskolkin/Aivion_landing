package lifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"
	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/integrations/ozon"
	"github.com/jackc/pgx/v5"
)

const PostingPageSize = 1000
const ReturnsPageSize = 500

type LifecycleAPI interface {
	ListCanonicalFBOPostings(context.Context, int64, ozon.PostingListRequest) (*ozon.TypedResponse[ozon.PostingListResult], error)
	ListCanonicalFBSPostings(context.Context, int64, ozon.PostingListRequest) (*ozon.TypedResponse[ozon.PostingListResult], error)
	GetCanonicalFBOPosting(context.Context, int64, ozon.PostingDetailRequest) (*ozon.TypedResponse[ozon.CanonicalPosting], error)
	GetCanonicalFBSPosting(context.Context, int64, ozon.PostingDetailRequest) (*ozon.TypedResponse[ozon.CanonicalPosting], error)
	ListCanonicalReturns(context.Context, int64, ozon.ReturnsListRequest) (*ozon.TypedResponse[ozon.ReturnsListResult], error)
}
type payloadRegistrar interface {
	StoreAndRegisterPayload(context.Context, *collection.SourcePayloadStore, collection.RegisterPayloadInput) (collection.SourcePayload, error)
}
type cursorStore interface {
	UpsertCursor(context.Context, int64, collection.CollectionDomain, string, []byte) (collection.Cursor, error)
}
type replayRepository interface {
	GetCursor(context.Context, int64, collection.CollectionDomain, string) (collection.Cursor, error)
	ListPayloads(context.Context, int64, int64) ([]collection.SourcePayload, error)
}
type Runner struct {
	API        LifecycleAPI
	Collection payloadRegistrar
	Payloads   *collection.SourcePayloadStore
	Cursors    cursorStore
	Recovery   replayRepository
}
type postingStream struct {
	name, endpoint, detailEndpoint, profile string
	contract, detailContract                collection.SourceContract
	fetch                                   func(context.Context, int64, ozon.PostingListRequest) (*ozon.TypedResponse[ozon.PostingListResult], error)
	detail                                  func(context.Context, int64, ozon.PostingDetailRequest) (*ozon.TypedResponse[ozon.CanonicalPosting], error)
}
type checkpoint struct {
	DomainRunID                            int64 `json:"collection_domain_run_id"`
	Stream, ContractKey, Version, From, To string
	Pages                                  int    `json:"pages"`
	NextCursor                             string `json:"next_cursor"`
	NextLastID                             int64  `json:"next_last_id"`
	Terminal                               bool   `json:"terminal"`
	Mode                                   string `json:"mode,omitempty"`
}
type pageMeta struct {
	Stream, ContractKey, Version, Profile, From, To, Cursor, NextCursor string
	LastID, NextLastID                                                  int64
	HasNext                                                             bool
	Records                                                             int
}

func Initial90DayRange(now time.Time) (time.Time, time.Time) {
	moscow, _ := time.LoadLocation("Europe/Moscow")
	local := now.In(moscow).AddDate(0, 0, -1)
	to := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
	return to.AddDate(0, 0, -89), to
}
func (r *Runner) validate() error {
	if r == nil || r.API == nil || r.Collection == nil || r.Payloads == nil {
		return errors.New("lifecycle runner is not configured")
	}
	return nil
}
func (r *Runner) CollectPostings(ctx context.Context, seller, run int64, from, to time.Time) (PostingsResult, error) {
	return r.CollectPostingsMode(ctx, seller, run, PostingModeExplicitRange, from, to)
}
func (r *Runner) CollectPostingsMode(ctx context.Context, seller, run int64, mode string, from, to time.Time) (PostingsResult, error) {
	out := PostingsResult{RequestedFrom: day(from), RequestedTo: day(to), Mode: mode}
	if err := r.validate(); err != nil {
		return out, err
	}
	if mode != PostingModeInitialRange && mode != PostingModeExplicitRange && mode != PostingModeIncrementalChangedStatus {
		return out, fmt.Errorf("invalid posting collection mode %q", mode)
	}
	seen := map[string]PostingCandidate{}
	streams := []postingStream{{"FBO", FBOListEndpoint, FBODetailEndpoint, "OZON_POSTINGS_FBO_REDACTION_V1", FBOListContract, FBODetailContract, r.API.ListCanonicalFBOPostings, r.API.GetCanonicalFBOPosting}, {"FBS", FBSListEndpoint, FBSDetailEndpoint, "OZON_POSTINGS_FBS_REDACTION_V1", FBSListContract, FBSDetailContract, r.API.ListCanonicalFBSPostings, r.API.GetCanonicalFBSPosting}}
	var permanentErr error
	for i := range streams {
		streamRows := map[string]PostingCandidate{}
		sr, err := r.collectPostingStream(ctx, seller, run, &out, streams[i], streamRows)
		if streams[i].name == "FBO" {
			out.FBO = sr
		} else {
			out.FBS = sr
		}
		if err != nil {
			if IsResumableInterruption(err) {
				sr.Status = "INTERRUPTED"
				sr.ErrorCode = ErrorCode(err)
				if streams[i].name == "FBO" {
					out.FBO = sr
				} else {
					out.FBS = sr
				}
				return out, err
			}
			var pe *ozon.ProviderError
			if errors.As(err, &pe) && pe.StatusCode >= 400 && pe.StatusCode < 500 {
				if pe.StatusCode == 401 || pe.StatusCode == 403 {
					sr.Status = "UNAVAILABLE"
				} else {
					sr.Status = "FAILED"
				}
				sr.ErrorCode = ErrorCode(err)
				if streams[i].name == "FBO" {
					out.FBO = sr
				} else {
					out.FBS = sr
				}
				permanentErr = err
				continue
			}
			return out, err
		}
		for k, v := range streamRows {
			seen[k] = v
		}
	}
	if !streamFinal(out.FBO.Status) && !streamFinal(out.FBS.Status) && permanentErr != nil {
		return out, permanentErr
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out.postings = append(out.postings, seen[k])
	}
	out.Postings = len(out.postings)
	return out, nil
}
func (r *Runner) collectPostingStream(ctx context.Context, seller, run int64, out *PostingsResult, s postingStream, seen map[string]PostingCandidate) (StreamResult, error) {
	sr := StreamResult{Status: "RUNNING"}
	details, err := r.loadPostingDetails(ctx, seller, run, s)
	if err != nil {
		return sr, err
	}
	cursor, page, terminal, resumed, err := r.replayPostings(ctx, seller, run, out, s, seen, details)
	sr.Resumed = resumed
	if err != nil {
		return sr, err
	}
	if terminal {
		sr.Status = "COMPLETE"
		sr.Pages = page - 1
		for _, v := range seen {
			if v.Scheme == s.name || (s.name == "FBS" && (v.Scheme == "RFBS" || v.Scheme == "UNKNOWN")) {
				sr.Records++
			}
		}
		if sr.Records == 0 {
			sr.Status = "EMPTY_VALID"
		}
		return sr, nil
	}
	visited := map[string]struct{}{cursor: {}}
	for !terminal {
		if err := ctx.Err(); err != nil {
			return sr, err
		}
		req := postingRequest(out, s.name, cursor)
		resp, err := s.fetch(ctx, seller, req)
		if err != nil {
			return sr, err
		}
		rows := append([]ozon.CanonicalPosting(nil), resp.Data.Postings...)
		for i := range rows {
			if NeedPostingDetail(rows[i]) {
				enriched, ok := details[rows[i].PostingNumber]
				if !ok {
					var req ozon.PostingDetailRequest
					req.PostingNumber = rows[i].PostingNumber
					detailResp, e := s.detail(ctx, seller, req)
					if e != nil {
						return sr, e
					}
					status := int32(detailResp.Meta.StatusCode)
					meta := map[string]any{"posting_number": rows[i].PostingNumber, "contract_key": s.detailContract.ContractKey, "contract_version": s.detailContract.ContractVersion, "redaction_profile": s.profile}
					p, e := r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: s.detailEndpoint, RequestIdentity: map[string]any{"posting_number": rows[i].PostingNumber}, Metadata: meta, Body: detailResp.Raw, ContentType: "application/json", HTTPStatus: &status})
					if e != nil {
						return sr, e
					}
					enriched = detailArtifact{posting: detailResp.Data, payload: p}
					details[rows[i].PostingNumber] = enriched
					sr.Details++
				}
				rows[i] = enriched.posting
			}
		}
		seq := int64(page)
		status := int32(resp.Meta.StatusCode)
		meta := pageMeta{Stream: s.name, ContractKey: s.contract.ContractKey, Version: s.contract.ContractVersion, Profile: s.profile, From: requestFrom(req), To: requestTo(req), Cursor: cursor, NextCursor: resp.Data.Cursor, HasNext: resp.Data.HasNext, Records: len(resp.Data.Postings)}
		payload, err := r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: s.endpoint, RequestIdentity: postingIdentity(req), Metadata: metaMap(meta), Body: resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
		if err != nil {
			return sr, err
		}
		if err = acceptPostingRowsDetailed(rows, s, payload, seen, details); err != nil {
			return sr, err
		}
		sr.Pages++
		sr.Records += len(resp.Data.Postings)
		terminal = !resp.Data.HasNext
		if !terminal {
			next := resp.Data.Cursor
			if next == "" || next == cursor {
				return sr, fmt.Errorf("%w: %s cursor did not advance", ErrPaginationNonAdvancing, s.name)
			}
			if _, ok := visited[next]; ok {
				return sr, fmt.Errorf("%w: %s repeated cursor", ErrPaginationNonAdvancing, s.name)
			}
			visited[next] = struct{}{}
			cursor = next
		}
		if r.Cursors != nil {
			v, _ := json.Marshal(checkpoint{DomainRunID: run, Stream: s.name, ContractKey: s.contract.ContractKey, Version: s.contract.ContractVersion, From: requestFrom(req), To: requestTo(req), Pages: sr.Pages, NextCursor: cursor, Terminal: terminal, Mode: out.Mode})
			if _, err = r.Cursors.UpsertCursor(ctx, seller, collection.DomainPostings, cursorKey(run, s.name), v); err != nil {
				return sr, err
			}
		}
		page++
	}
	sr.Status = "COMPLETE"
	if sr.Records == 0 {
		sr.Status = "EMPTY_VALID"
	}
	return sr, nil
}
func acceptPostingRows(rows []ozon.CanonicalPosting, s postingStream, payload collection.SourcePayload, seen map[string]PostingCandidate) error {
	return acceptPostingRowsDetailed(rows, s, payload, seen, nil)
}
func acceptPostingRowsDetailed(rows []ozon.CanonicalPosting, s postingStream, payload collection.SourcePayload, seen map[string]PostingCandidate, details map[string]detailArtifact) error {
	for _, raw := range rows {
		provenance := payload
		if d, ok := details[raw.PostingNumber]; ok {
			provenance = d.payload
		}
		c, err := normalizePosting(raw, s.name, provenance.ID, provenance.ReceivedAt)
		if err != nil {
			return err
		}
		if previous, ok := seen[c.PostingNumber]; ok {
			if previous.OrderID != c.OrderID || previous.OrderNumber != c.OrderNumber {
				return fmt.Errorf("%w: order %d", ErrOrderIdentityConflict, c.OrderID)
			}
			if previous.Scheme != "UNKNOWN" && c.Scheme != "UNKNOWN" && previous.Scheme != c.Scheme {
				return fmt.Errorf("%w: posting %s", ErrPostingSchemeConflict, c.PostingNumber)
			}
			return fmt.Errorf("%w: duplicate posting %s", ErrSourceContractMismatch, c.PostingNumber)
		}
		seen[c.PostingNumber] = c
	}
	return nil
}
func (r *Runner) replayPostings(ctx context.Context, seller, run int64, out *PostingsResult, s postingStream, seen map[string]PostingCandidate, details map[string]detailArtifact) (string, int, bool, bool, error) {
	if r.Recovery == nil {
		return "", 1, false, false, nil
	}
	all, err := r.Recovery.ListPayloads(ctx, seller, run)
	if err != nil {
		return "", 0, false, false, err
	}
	var artifacts []collection.SourcePayload
	for _, p := range all {
		if p.EndpointKey == s.endpoint {
			artifacts = append(artifacts, p)
		}
	}
	cur, err := r.Recovery.GetCursor(ctx, seller, collection.DomainPostings, cursorKey(run, s.name))
	if errors.Is(err, pgx.ErrNoRows) {
		if len(artifacts) > 0 {
			return "", 0, false, false, integrity(s.name, "artifacts without checkpoint")
		}
		return "", 1, false, false, nil
	}
	if err != nil {
		return "", 0, false, false, err
	}
	var cp checkpoint
	if json.Unmarshal(cur.Value, &cp) != nil || cp.DomainRunID != run || cp.Stream != s.name || cp.ContractKey != s.contract.ContractKey || cp.Version != s.contract.ContractVersion || cp.From != out.RequestedFrom.Format(time.RFC3339) || cp.To != out.RequestedTo.Add(24*time.Hour-time.Nanosecond).Format(time.RFC3339Nano) || cp.Mode != out.Mode {
		return "", 0, false, false, integrity(s.name, "checkpoint mismatch")
	}
	if len(artifacts) != cp.Pages {
		return "", 0, false, false, integrity(s.name, "page count mismatch")
	}
	sortPayloads(artifacts)
	expected := ""
	for i, p := range artifacts {
		if err := ctx.Err(); err != nil {
			return "", 0, false, false, err
		}
		if p.PageSeq == nil || *p.PageSeq != int64(i+1) || p.RedactionProfile != s.profile {
			return "", 0, false, false, integrity(s.name, "sequence/profile mismatch")
		}
		var m pageMeta
		if json.Unmarshal(p.Metadata, &m) != nil || m.Stream != s.name || m.ContractKey != s.contract.ContractKey || m.Version != s.contract.ContractVersion || m.Cursor != expected {
			return "", 0, false, false, integrity(s.name, "metadata continuity mismatch")
		}
		req := postingRequest(out, s.name, expected)
		fp, _ := collection.RequestFingerprint(collection.ProviderOzon, s.endpoint, withDomain(postingIdentity(req), collection.DomainPostings))
		if fp != p.RequestFingerprint {
			return "", 0, false, false, integrity(s.name, "fingerprint mismatch")
		}
		ok, e := r.Payloads.Verify(ctx, p)
		if e != nil || !ok {
			return "", 0, false, false, integrity(s.name, "artifact hash mismatch")
		}
		body, e := r.Payloads.Get(ctx, p.Bucket, p.ObjectKey)
		if e != nil {
			return "", 0, false, false, integrity(s.name, "artifact missing")
		}
		parsed, e := ozon.ParsePostingListResponse(body)
		if e != nil || len(parsed.Postings) != m.Records {
			return "", 0, false, false, integrity(s.name, "decode mismatch")
		}
		rows := append([]ozon.CanonicalPosting(nil), parsed.Postings...)
		for j := range rows {
			if NeedPostingDetail(rows[j]) {
				d, ok := details[rows[j].PostingNumber]
				if !ok {
					return "", 0, false, false, integrity(s.name, "missing required detail for %s", rows[j].PostingNumber)
				}
				rows[j] = d.posting
			}
		}
		if e = acceptPostingRowsDetailed(rows, s, p, seen, details); e != nil {
			return "", 0, false, false, integrity(s.name, "normalize: %v", e)
		}
		if m.HasNext {
			if m.NextCursor == "" || m.NextCursor == expected {
				return "", 0, false, false, integrity(s.name, "non-advancing cursor")
			}
			expected = m.NextCursor
		}
	}
	if expected != cp.NextCursor {
		return "", 0, false, false, integrity(s.name, "checkpoint cursor mismatch")
	}
	return cp.NextCursor, cp.Pages + 1, cp.Terminal, true, nil
}

type detailArtifact struct {
	posting ozon.CanonicalPosting
	payload collection.SourcePayload
}

func (r *Runner) loadPostingDetails(ctx context.Context, seller, run int64, s postingStream) (map[string]detailArtifact, error) {
	out := map[string]detailArtifact{}
	if r.Recovery == nil {
		return out, nil
	}
	payloads, e := r.Recovery.ListPayloads(ctx, seller, run)
	if e != nil {
		return nil, e
	}
	for _, p := range payloads {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if p.EndpointKey != s.detailEndpoint {
			continue
		}
		if p.RedactionProfile != s.profile {
			return nil, integrity(s.name, "detail redaction mismatch")
		}
		var m struct {
			PostingNumber   string `json:"posting_number"`
			ContractKey     string `json:"contract_key"`
			ContractVersion string `json:"contract_version"`
		}
		if json.Unmarshal(p.Metadata, &m) != nil || m.PostingNumber == "" || m.ContractKey != s.detailContract.ContractKey || m.ContractVersion != s.detailContract.ContractVersion {
			return nil, integrity(s.name, "detail metadata mismatch")
		}
		fp, _ := collection.RequestFingerprint(collection.ProviderOzon, s.detailEndpoint, withDomain(map[string]any{"posting_number": m.PostingNumber}, collection.DomainPostings))
		if fp != p.RequestFingerprint {
			return nil, integrity(s.name, "detail fingerprint mismatch")
		}
		ok, er := r.Payloads.Verify(ctx, p)
		if er != nil || !ok {
			return nil, integrity(s.name, "detail hash mismatch")
		}
		body, er := r.Payloads.Get(ctx, p.Bucket, p.ObjectKey)
		if er != nil {
			return nil, integrity(s.name, "detail missing")
		}
		posting, er := ozon.ParsePostingDetailResponse(body)
		if er != nil || posting.PostingNumber != m.PostingNumber {
			return nil, integrity(s.name, "detail decode/identity mismatch")
		}
		if _, exists := out[m.PostingNumber]; exists {
			return nil, integrity(s.name, "duplicate detail artifact")
		}
		out[m.PostingNumber] = detailArtifact{posting: posting, payload: p}
	}
	return out, nil
}
func (r *Runner) CollectReturns(ctx context.Context, seller, run int64, from, to time.Time) (ReturnsResult, error) {
	out := ReturnsResult{RequestedFrom: day(from), RequestedTo: day(to)}
	if err := r.validate(); err != nil {
		return out, err
	}
	seen := map[int64]ReturnCandidate{}
	last, page, terminal, resumed, err := r.replayReturns(ctx, seller, run, &out, seen)
	out.Stream.Resumed = resumed
	if err != nil {
		return out, err
	}
	visited := map[int64]struct{}{last: {}}
	for !terminal {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		req := returnsRequest(&out, last)
		resp, err := r.API.ListCanonicalReturns(ctx, seller, req)
		if err != nil {
			return out, err
		}
		seq := int64(page)
		status := int32(resp.Meta.StatusCode)
		next := int64(resp.Data.LastID)
		if resp.Data.HasNext && (next <= last) {
			return out, fmt.Errorf("%w: returns last_id", ErrPaginationNonAdvancing)
		}
		meta := pageMeta{Stream: "RETURNS", ContractKey: ReturnsListContract.ContractKey, Version: ReturnsListContract.ContractVersion, Profile: "OZON_RETURNS_REDACTION_V1", From: req.Filter.VisualStatusChangeMoment.TimeFrom, To: req.Filter.VisualStatusChangeMoment.TimeTo, LastID: last, NextLastID: next, HasNext: resp.Data.HasNext, Records: len(resp.Data.Returns)}
		payload, err := r.Collection.StoreAndRegisterPayload(ctx, r.Payloads, collection.RegisterPayloadInput{SellerAccountID: seller, DomainRunID: run, Provider: collection.ProviderOzon, EndpointKey: ReturnsEndpoint, RequestIdentity: returnsIdentity(req), Metadata: metaMap(meta), Body: resp.Raw, ContentType: "application/json", PageSeq: &seq, HTTPStatus: &status})
		if err != nil {
			return out, err
		}
		if err = acceptReturnRows(resp.Data.Returns, payload, seen); err != nil {
			return out, err
		}
		out.Stream.Pages++
		out.Stream.Records += len(resp.Data.Returns)
		terminal = !resp.Data.HasNext
		if !terminal {
			if _, ok := visited[next]; ok {
				return out, fmt.Errorf("%w: repeated returns last_id", ErrPaginationNonAdvancing)
			}
			visited[next] = struct{}{}
			last = next
		}
		if r.Cursors != nil {
			v, _ := json.Marshal(checkpoint{DomainRunID: run, Stream: "RETURNS", ContractKey: ReturnsListContract.ContractKey, Version: ReturnsListContract.ContractVersion, From: meta.From, To: meta.To, Pages: out.Stream.Pages, NextLastID: last, Terminal: terminal})
			if _, err = r.Cursors.UpsertCursor(ctx, seller, collection.DomainReturns, cursorKey(run, "RETURNS"), v); err != nil {
				return out, err
			}
		}
		page++
	}
	keys := make([]int64, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		out.returns = append(out.returns, seen[k])
	}
	out.Returns = len(out.returns)
	out.Stream.Status = "COMPLETE"
	return out, nil
}
func acceptReturnRows(rows []ozon.CanonicalReturn, p collection.SourcePayload, seen map[int64]ReturnCandidate) error {
	for _, raw := range rows {
		c, e := normalizeReturn(raw, p.ID, p.ReceivedAt)
		if e != nil {
			return e
		}
		if _, ok := seen[c.ProviderReturnID]; ok {
			return fmt.Errorf("%w: duplicate return %d", ErrSourceContractMismatch, c.ProviderReturnID)
		}
		seen[c.ProviderReturnID] = c
	}
	return nil
}
func (r *Runner) replayReturns(ctx context.Context, seller, run int64, out *ReturnsResult, seen map[int64]ReturnCandidate) (int64, int, bool, bool, error) {
	if r.Recovery == nil {
		return 0, 1, false, false, nil
	}
	all, e := r.Recovery.ListPayloads(ctx, seller, run)
	if e != nil {
		return 0, 0, false, false, e
	}
	var a []collection.SourcePayload
	for _, p := range all {
		if p.EndpointKey == ReturnsEndpoint {
			a = append(a, p)
		}
	}
	cur, e := r.Recovery.GetCursor(ctx, seller, collection.DomainReturns, cursorKey(run, "RETURNS"))
	if errors.Is(e, pgx.ErrNoRows) {
		if len(a) > 0 {
			return 0, 0, false, false, integrity("RETURNS", "artifacts without checkpoint")
		}
		return 0, 1, false, false, nil
	}
	if e != nil {
		return 0, 0, false, false, e
	}
	var cp checkpoint
	if json.Unmarshal(cur.Value, &cp) != nil || cp.DomainRunID != run || cp.ContractKey != ReturnsListContract.ContractKey {
		return 0, 0, false, false, integrity("RETURNS", "checkpoint mismatch")
	}
	if len(a) != cp.Pages {
		return 0, 0, false, false, integrity("RETURNS", "page count mismatch")
	}
	sortPayloads(a)
	last := int64(0)
	for i, p := range a {
		if p.PageSeq == nil || *p.PageSeq != int64(i+1) || p.RedactionProfile != "OZON_RETURNS_REDACTION_V1" {
			return 0, 0, false, false, integrity("RETURNS", "sequence/profile mismatch")
		}
		var m pageMeta
		if json.Unmarshal(p.Metadata, &m) != nil || m.LastID != last {
			return 0, 0, false, false, integrity("RETURNS", "metadata continuity mismatch")
		}
		req := returnsRequest(out, last)
		fp, _ := collection.RequestFingerprint(collection.ProviderOzon, ReturnsEndpoint, withDomain(returnsIdentity(req), collection.DomainReturns))
		if fp != p.RequestFingerprint {
			return 0, 0, false, false, integrity("RETURNS", "fingerprint mismatch")
		}
		ok, er := r.Payloads.Verify(ctx, p)
		if er != nil || !ok {
			return 0, 0, false, false, integrity("RETURNS", "hash mismatch")
		}
		body, er := r.Payloads.Get(ctx, p.Bucket, p.ObjectKey)
		if er != nil {
			return 0, 0, false, false, integrity("RETURNS", "missing artifact")
		}
		parsed, er := ozon.ParseReturnsListResponse(body)
		if er != nil || len(parsed.Returns) != m.Records {
			return 0, 0, false, false, integrity("RETURNS", "decode mismatch")
		}
		if er = acceptReturnRows(parsed.Returns, p, seen); er != nil {
			return 0, 0, false, false, integrity("RETURNS", "normalize: %v", er)
		}
		if m.HasNext {
			if m.NextLastID <= last {
				return 0, 0, false, false, integrity("RETURNS", "non-advancing last_id")
			}
			last = m.NextLastID
		}
	}
	if last != cp.NextLastID {
		return 0, 0, false, false, integrity("RETURNS", "checkpoint last_id mismatch")
	}
	return last, cp.Pages + 1, cp.Terminal, true, nil
}
func postingRequest(out *PostingsResult, stream, cursor string) ozon.PostingListRequest {
	var r ozon.PostingListRequest
	r.Dir = "ASC"
	from, to := out.RequestedFrom.Format(time.RFC3339), out.RequestedTo.Add(24*time.Hour-time.Nanosecond).Format(time.RFC3339Nano)
	if stream == "FBS" && out.Mode == PostingModeIncrementalChangedStatus {
		r.Filter.LastChangedStatusDate = &struct {
			From string `json:"from"`
			To   string `json:"to"`
		}{From: from, To: to}
	} else {
		r.Filter.Since, r.Filter.To = from, to
	}
	r.Limit = PostingPageSize
	r.Cursor = cursor
	return r
}
func returnsRequest(out *ReturnsResult, last int64) ozon.ReturnsListRequest {
	var r ozon.ReturnsListRequest
	r.Filter.VisualStatusChangeMoment = &ozon.ReturnWindow{TimeFrom: out.RequestedFrom.Format(time.RFC3339), TimeTo: out.RequestedTo.Add(24*time.Hour - time.Nanosecond).Format(time.RFC3339Nano)}
	r.Limit = ReturnsPageSize
	r.LastID = last
	return r
}
func postingIdentity(r ozon.PostingListRequest) map[string]any {
	m := map[string]any{"since": r.Filter.Since, "to": r.Filter.To, "limit": r.Limit, "cursor": r.Cursor, "financial_data": false, "barcodes": false}
	if r.Filter.LastChangedStatusDate != nil {
		m["last_changed_status_from"] = r.Filter.LastChangedStatusDate.From
		m["last_changed_status_to"] = r.Filter.LastChangedStatusDate.To
	}
	return m
}
func requestFrom(r ozon.PostingListRequest) string {
	if r.Filter.LastChangedStatusDate != nil {
		return r.Filter.LastChangedStatusDate.From
	}
	return r.Filter.Since
}
func requestTo(r ozon.PostingListRequest) string {
	if r.Filter.LastChangedStatusDate != nil {
		return r.Filter.LastChangedStatusDate.To
	}
	return r.Filter.To
}

func NeedPostingDetail(p ozon.CanonicalPosting) bool {
	return strings.TrimSpace(p.PostingNumber) != "" && (strings.TrimSpace(p.OrderNumber) == "" || int64(p.OrderID) <= 0 || len(p.Products) == 0)
}
func returnsIdentity(r ozon.ReturnsListRequest) map[string]any {
	return map[string]any{"visual_status_change_from": r.Filter.VisualStatusChangeMoment.TimeFrom, "visual_status_change_to": r.Filter.VisualStatusChangeMoment.TimeTo, "limit": r.Limit, "last_id": r.LastID}
}
func withDomain(m map[string]any, d collection.CollectionDomain) map[string]any {
	c := map[string]any{}
	for k, v := range m {
		c[k] = v
	}
	c["collection_domain"] = string(d)
	return c
}
func metaMap(m pageMeta) map[string]any {
	raw, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return out
}
func cursorKey(run int64, stream string) string { return fmt.Sprintf("run:%d:%s", run, stream) }
func integrity(stream, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrResumeArtifactIntegrity, stream, fmt.Sprintf(format, args...))
}
func sortPayloads(a []collection.SourcePayload) {
	sort.Slice(a, func(i, j int) bool {
		if a[i].PageSeq == nil {
			return false
		}
		if a[j].PageSeq == nil {
			return true
		}
		return *a[i].PageSeq < *a[j].PageSeq
	})
}
func day(v time.Time) time.Time { return time.Date(v.Year(), v.Month(), v.Day(), 0, 0, 0, 0, time.UTC) }
