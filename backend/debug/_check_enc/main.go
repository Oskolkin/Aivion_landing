package main

import (
	"fmt"
	"os"

	"github.com/Oskolkin/marketplace-ai-mvp/backend/internal/config"
)

func main() {
	_ = config.LoadEnvFiles()
	k := os.Getenv("ENCRYPTION_KEY")
	fmt.Printf("env_len=%d set=%v\n", len(k), k != "")
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("load_err=%v\n", err)
		return
	}
	fmt.Printf("cfg_enc_len=%d\n", len(cfg.Auth.EncryptionKey))
}
