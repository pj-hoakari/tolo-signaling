// Package config はシグナリングサーバの設定を環境変数から読み込む。
package config

import "os"

// Config is the configuration for the signaling server.
type Config struct {
	// Addr is the address to listen on (e.g. ":8787"). Defaults to ":8787".
	Addr string
	// Firestore is the configuration for Firestore backend.
	Firestore Firestore
}

// Firestore is the configuration for Firestore backend.
type Firestore struct {
	// ProjectID is the GCP project ID (must match the client writing to presence).
	ProjectID string
	// EmulatorHost is the host and port of the Firestore emulator.
	// When EmulatorHost is set, the server will connect to the Firestore emulator instead of the real Firestore service.
	EmulatorHost string
}

func Load() Config {
	return Config{
		Addr: getenv("SIGNALING_ADDR", ":8787"),
		Firestore: Firestore{
			ProjectID:    getenv("FIRESTORE_PROJECT_ID", "tolo-signaling"),
			EmulatorHost: getenv("FIRESTORE_EMULATOR_HOST", "127.0.0.1:8080"),
		},
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
