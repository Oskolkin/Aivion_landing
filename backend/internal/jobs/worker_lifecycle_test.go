package jobs

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

func TestRunAsynqUntilContext_gracefulShutdown(t *testing.T) {
	redisAddr := getenvDefault("TEST_REDIS_ADDR", "localhost:6379")
	cfg := RedisConfig{Addr: redisAddr, DB: 15}

	client := NewAsynqClient(cfg)
	defer client.Close()

	server := NewAsynqServer(cfg)
	mux := asynq.NewServeMux()
	pingDone := make(chan string, 1)
	mux.HandleFunc(TypeSystemPing, func(ctx context.Context, task *asynq.Task) error {
		var payload SystemPingPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		pingDone <- payload.Message
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- RunAsynqUntilContext(ctx, server, mux)
	}()

	task, err := NewSystemPingTask("lifecycle-ping")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Enqueue(task); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	select {
	case msg := <-pingDone:
		if msg != "lifecycle-ping" {
			t.Fatalf("message=%q", msg)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for ping task")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("shutdown err: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("timeout waiting for server shutdown")
	}
}

func TestRunAsynqUntilContext_immediateCancel(t *testing.T) {
	redisAddr := getenvDefault("TEST_REDIS_ADDR", "localhost:6379")
	cfg := RedisConfig{Addr: redisAddr, DB: 15}
	server := NewAsynqServer(cfg)
	mux := asynq.NewServeMux()

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	if err := RunAsynqUntilContext(ctx, server, mux); err != context.Canceled {
		t.Fatalf("err=%v", err)
	}
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
