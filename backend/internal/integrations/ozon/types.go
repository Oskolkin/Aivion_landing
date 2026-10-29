package ozon

type ResponseMeta struct {
	StatusCode int
	RequestID  string
}

type RawResponse struct {
	Body []byte
	Meta ResponseMeta
}

type TypedResponse[T any] struct {
	Raw  []byte
	Data T
	Meta ResponseMeta
}
