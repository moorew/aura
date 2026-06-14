package db

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func newTestConfigStore(t *testing.T) *IntegrationConfigStore {
	t.Helper()
	conn, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := Migrate(conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return NewIntegrationConfigStore(conn)
}

func TestResolveAITitleEnvDefault(t *testing.T) {
	s := newTestConfigStore(t)
	ctx := context.Background()

	// No stored config → enabled from env when a base URL is present.
	got := s.ResolveAITitle(ctx, "http://ollama:11434", "qwen2.5:1.5b")
	if !got.Enabled || got.BaseURL != "http://ollama:11434" || got.Model != "qwen2.5:1.5b" {
		t.Fatalf("env default mismatch: %+v", got)
	}

	// No env, no config → disabled.
	if s.ResolveAITitle(ctx, "", "").Enabled {
		t.Fatal("expected disabled with no env base URL and no stored config")
	}
}

func TestResolveAITitleDBOverride(t *testing.T) {
	s := newTestConfigStore(t)
	ctx := context.Background()

	cfgJSON, _ := json.Marshal(AITitleConfig{BaseURL: "http://host:9999", Model: "llama3"})
	if _, err := s.Upsert(ctx, uuid.New().String(), AITitleType, string(cfgJSON)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.SetEnabled(ctx, AITitleType, true); err != nil {
		t.Fatalf("set enabled: %v", err)
	}

	got := s.ResolveAITitle(ctx, "http://ollama:11434", "envmodel")
	if !got.Enabled || got.BaseURL != "http://host:9999" || got.Model != "llama3" {
		t.Fatalf("db override mismatch: %+v", got)
	}

	// Disabling flips Enabled off; the saved values are retained.
	if err := s.SetEnabled(ctx, AITitleType, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if s.ResolveAITitle(ctx, "http://ollama:11434", "envmodel").Enabled {
		t.Fatal("expected disabled after SetEnabled(false)")
	}
}

func TestResolveAITitleEmptyFieldsFallBackToEnv(t *testing.T) {
	s := newTestConfigStore(t)
	ctx := context.Background()

	cfgJSON, _ := json.Marshal(AITitleConfig{BaseURL: "", Model: ""})
	if _, err := s.Upsert(ctx, uuid.New().String(), AITitleType, string(cfgJSON)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if err := s.SetEnabled(ctx, AITitleType, true); err != nil {
		t.Fatalf("set enabled: %v", err)
	}

	got := s.ResolveAITitle(ctx, "http://env:11434", "envmodel")
	if got.BaseURL != "http://env:11434" || got.Model != "envmodel" {
		t.Fatalf("expected env fallback for empty stored fields: %+v", got)
	}
}
