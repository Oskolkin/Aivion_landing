package lifecycle

import "github.com/Oskolkin/marketplace-ai-mvp/backend/internal/collection"

var FBOListContract = collection.NewSourceContract("POSTINGS_FBO", "OZON:POSTINGS:FBO_LIST_V3", "1.0.0", "POST /v3/posting/fbo/list", "canonical-postings-fbo-list-v1", "seller+posting_number; order=order_id; cursor+has_next; products=version-composition; no-finance")
var FBODetailContract = collection.NewSourceContract("POSTINGS_FBO", "OZON:POSTINGS:FBO_GET_V2", "1.0.0", "POST /v2/posting/fbo/get", "canonical-postings-fbo-get-v1", "detail-by-posting_number; required-lifecycle-enrichment; no-finance")
var FBSListContract = collection.NewSourceContract("POSTINGS_FBS", "OZON:POSTINGS:FBS_LIST_V4", "1.0.0", "POST /v4/posting/fbs/list", "canonical-postings-fbs-list-v1", "seller+posting_number; explicit-delivery-schema; explicit-parent-lineage; cursor+has_next; no-finance")
var FBSDetailContract = collection.NewSourceContract("POSTINGS_FBS", "OZON:POSTINGS:FBS_GET_V3", "1.0.0", "POST /v3/posting/fbs/get", "canonical-postings-fbs-get-v1", "detail-by-posting_number; required-lifecycle-enrichment; no-finance")
var ReturnsListContract = collection.NewSourceContract("RETURNS", "OZON:RETURNS:LIST_V1", "1.0.0", "POST /v1/returns/list", "canonical-returns-list-v1", "identity=returns.id; visual_status_change_moment; last_id+has_next; limit<=500; preserve-provider-semantics")

var PostingsDomainContract = collection.NewSourceContract("POSTINGS", "OZON:POSTINGS:CANONICAL_V1", "1.0.0", "FBO_V3+FBO_GET_V2+FBS_V4+FBS_GET_V3", "canonical-postings-v1", FBOListContract.SemanticMappingHash+FBODetailContract.SemanticMappingHash+FBSListContract.SemanticMappingHash+FBSDetailContract.SemanticMappingHash)
var ReturnsDomainContract = collection.NewSourceContract("RETURNS", "OZON:RETURNS:CANONICAL_V1", "1.0.0", "RETURNS_LIST_V1", "canonical-returns-v1", ReturnsListContract.SemanticMappingHash)

const (
	FBOListEndpoint   = "ozon.postings.fbo.list"
	FBODetailEndpoint = "ozon.postings.fbo.get"
	FBSListEndpoint   = "ozon.postings.fbs.list"
	FBSDetailEndpoint = "ozon.postings.fbs.get"
	ReturnsEndpoint   = "ozon.returns.list"
)
