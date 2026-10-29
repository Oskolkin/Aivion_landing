package provider

import "context"

// Provider abstracts OpenAI Containers, Container Files, and background Responses.
type Provider interface {
	CreateContainer(ctx context.Context, req CreateContainerRequest) (*Container, error)
	RetrieveContainer(ctx context.Context, containerID string) (*Container, error)
	UploadContainerFile(ctx context.Context, req UploadContainerFileRequest) (*ContainerFile, error)
	StartBackgroundResponse(ctx context.Context, req StartBackgroundResponseRequest) (*ProviderResponse, error)
	RetrieveResponse(ctx context.Context, responseID string) (*ProviderResponse, error)
	CancelResponse(ctx context.Context, responseID string) (*ProviderResponse, error)
	DeleteContainerFile(ctx context.Context, containerID, fileID string) error
	DeleteContainer(ctx context.Context, containerID string) error
}
