package profile

import (
	"encoding/json"
	"testing"
)

func TestPatchProfileRequestRejectsBalance(t *testing.T) {
	var req PatchProfileRequest
	if err := json.Unmarshal([]byte(`{"name":"user","balance":1000}`), &req); err == nil {
		t.Fatal("expected balance field to be rejected")
	}
}

func TestPatchProfileRequestStillParsesRegularFields(t *testing.T) {
	var req PatchProfileRequest
	if err := json.Unmarshal([]byte(`{"name":"user","telegram":"@user"}`), &req); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if req.Name == nil || *req.Name != "user" {
		t.Fatalf("unexpected name: %#v", req.Name)
	}
	if !req.TelegramSet || req.Telegram == nil || *req.Telegram != "@user" {
		t.Fatalf("unexpected telegram state: set=%v value=%#v", req.TelegramSet, req.Telegram)
	}
}
