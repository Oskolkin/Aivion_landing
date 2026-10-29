package provider

import "context"

// CleanupContainer deletes the container file then the container.
// 404 responses are treated as idempotent success.
func CleanupContainer(ctx context.Context, p Provider, containerID, fileID string) error {
	if p == nil {
		return newProviderError(CodeProviderCleanupFailed, "cleanup container: nil provider", 0, "", nil)
	}
	if fileID != "" {
		if err := p.DeleteContainerFile(ctx, containerID, fileID); err != nil && !IsNotFound(err) {
			return newProviderError(CodeProviderCleanupFailed, "cleanup container file: "+err.Error(), 0, "", err)
		}
	}
	if containerID != "" {
		if err := p.DeleteContainer(ctx, containerID); err != nil && !IsNotFound(err) {
			return newProviderError(CodeProviderCleanupFailed, "cleanup container: "+err.Error(), 0, "", err)
		}
	}
	return nil
}
