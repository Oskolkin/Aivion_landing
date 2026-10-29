package collection

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type serviceRepository interface {
	GetRun(context.Context, int64, int64) (CollectionRun, error)
	TransitionRun(context.Context, int64, int64, CollectionRunStatus, CollectionRunStatus) (CollectionRun, error)
	GetDomainRun(context.Context, int64, int64) (DomainRun, error)
	TransitionDomainRun(context.Context, int64, int64, DomainRunStatus, DomainRunStatus) (DomainRun, error)
	UpsertCursor(context.Context, int64, CollectionDomain, string, []byte) (Cursor, error)
	CreatePayload(context.Context, SourcePayload) (SourcePayload, error)
}

type Service struct {
	repo              serviceRepository
	redactors         map[CollectionDomain]PayloadRedactor
	endpointRedactors map[string]PayloadRedactor
}

func NewService(repo serviceRepository) *Service {
	return &Service{repo: repo, redactors: defaultRedactors(), endpointRedactors: defaultEndpointRedactors()}
}
func (s *Service) WithEndpointPayloadRedactor(endpoint string, redactor PayloadRedactor) *Service {
	s.endpointRedactors[endpoint] = redactor
	return s
}

func (s *Service) WithPayloadRedactor(domain CollectionDomain, redactor PayloadRedactor) *Service {
	s.redactors[domain] = redactor
	return s
}
func (s *Service) TransitionRun(ctx context.Context, sellerID, id int64, to CollectionRunStatus) (CollectionRun, error) {
	current, err := s.repo.GetRun(ctx, sellerID, id)
	if err != nil {
		return CollectionRun{}, err
	}
	if err := ValidateRunTransition(current.Status, to); err != nil {
		if current.Status == to {
			return current, nil
		}
		return CollectionRun{}, err
	}
	return s.repo.TransitionRun(ctx, sellerID, id, current.Status, to)
}
func (s *Service) TransitionDomainRun(ctx context.Context, sellerID, id int64, to DomainRunStatus) (DomainRun, error) {
	current, err := s.repo.GetDomainRun(ctx, sellerID, id)
	if err != nil {
		return DomainRun{}, err
	}
	if err := ValidateDomainTransition(current.Status, to); err != nil {
		if current.Status == to {
			return current, nil
		}
		return DomainRun{}, err
	}
	return s.repo.TransitionDomainRun(ctx, sellerID, id, current.Status, to)
}
func (s *Service) UpsertCursor(ctx context.Context, sellerID int64, domain CollectionDomain, key string, value []byte) (Cursor, error) {
	return s.repo.UpsertCursor(ctx, sellerID, domain, key, value)
}

type RegisterPayloadInput struct {
	SellerAccountID, DomainRunID int64
	Provider                     Provider
	EndpointKey                  string
	RequestIdentity, Metadata    map[string]any
	Body                         []byte
	ContentType                  string
	PageSeq                      *int64
	HTTPStatus                   *int32
}

// StoreAndRegisterPayload writes the immutable object first. Therefore a
// successful provenance row cannot point to an object that this operation did
// not successfully write. A DB failure may leave a harmless unreferenced
// object, which is preferable to a broken provenance reference.
func (s *Service) StoreAndRegisterPayload(ctx context.Context, store *SourcePayloadStore, input RegisterPayloadInput) (SourcePayload, error) {
	domainRun, err := s.repo.GetDomainRun(ctx, input.SellerAccountID, input.DomainRunID)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("get source payload domain run: %w", err)
	}
	redactor, ok := s.endpointRedactors[input.EndpointKey]
	if !ok {
		redactor, ok = s.redactors[domainRun.Domain]
	}
	if !ok || redactor == nil {
		return SourcePayload{}, fmt.Errorf("no payload redaction policy for domain %s", domainRun.Domain)
	}
	redactedBody, err := redactor.Redact(ctx, input.EndpointKey, input.Body)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("redact source payload: %w", err)
	}
	requestIdentity := copyStringAnyMap(input.RequestIdentity)
	requestIdentity["collection_domain"] = string(domainRun.Domain)
	fingerprint, err := RequestFingerprint(input.Provider, input.EndpointKey, requestIdentity)
	if err != nil {
		return SourcePayload{}, fmt.Errorf("fingerprint source payload: %w", err)
	}
	metadataInput := copyStringAnyMap(input.Metadata)
	metadataInput["collection_domain"] = string(domainRun.Domain)
	metadataInput["redaction_profile"] = redactor.Profile()
	metadataInput["domain_source_contract_key"] = domainRun.SourceContractKey
	metadataInput["domain_source_contract_version"] = domainRun.SourceContractVersion
	metadataInput["domain_semantic_mapping_version"] = domainRun.SemanticMappingVersion
	metadataInput["domain_semantic_mapping_hash"] = domainRun.SemanticMappingHash
	if _, ok := metadataInput["source_contract_key"]; !ok {
		if v, present := metadataInput["contract_key"]; present {
			metadataInput["source_contract_key"] = v
		} else {
			metadataInput["source_contract_key"] = domainRun.SourceContractKey
		}
	}
	if _, ok := metadataInput["source_contract_version"]; !ok {
		if v, present := metadataInput["contract_version"]; present {
			metadataInput["source_contract_version"] = v
		} else {
			metadataInput["source_contract_version"] = domainRun.SourceContractVersion
		}
	}
	if _, ok := metadataInput["semantic_mapping_version"]; !ok {
		metadataInput["semantic_mapping_version"] = domainRun.SemanticMappingVersion
	}
	if _, ok := metadataInput["semantic_mapping_hash"]; !ok {
		metadataInput["semantic_mapping_hash"] = domainRun.SemanticMappingHash
	}
	metadata, err := json.Marshal(SanitizeMetadata(metadataInput))
	if err != nil {
		return SourcePayload{}, fmt.Errorf("sanitize source payload metadata: %w", err)
	}
	stored, err := store.Put(ctx, input.SellerAccountID, domainRun.Domain, redactedBody, input.ContentType)
	if err != nil {
		return SourcePayload{}, err
	}
	return s.repo.CreatePayload(ctx, SourcePayload{SellerAccountID: input.SellerAccountID, DomainRunID: input.DomainRunID, Provider: input.Provider, EndpointKey: input.EndpointKey, RequestFingerprint: fingerprint, Bucket: stored.Bucket, ObjectKey: stored.ObjectKey, SHA256: stored.SHA256, ReceivedAt: time.Now().UTC(), Metadata: metadata, RedactionProfile: redactor.Profile(), PageSeq: input.PageSeq, HTTPStatus: input.HTTPStatus})
}

func copyStringAnyMap(source map[string]any) map[string]any {
	result := make(map[string]any, len(source)+1)
	for key, value := range source {
		result[key] = value
	}
	return result
}
