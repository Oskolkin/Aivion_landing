package jobs

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"
)

// RunAsynqUntilContext runs the Asynq server until ctx is cancelled, then shuts down gracefully.
func RunAsynqUntilContext(ctx context.Context, server *asynq.Server, mux *asynq.ServeMux) error {
	if server == nil || mux == nil {
		return errors.New("asynq server and mux are required")
	}

	errCh := make(chan error, 1)
	go func() {
		if err := server.Run(mux); err != nil && !errors.Is(err, asynq.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		server.Shutdown()
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		default:
		}
		return ctx.Err()
	case err := <-errCh:
		server.Shutdown()
		return err
	}
}
