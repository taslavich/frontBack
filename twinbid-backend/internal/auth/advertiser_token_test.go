package auth

import (
	"regexp"
	"testing"
)

func TestGenerateAdvertiserAPITokenFormat(t *testing.T) {
	token, err := generateAdvertiserAPIToken()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if ok, _ := regexp.MatchString(`^adv_[0-9a-f]{64}$`, token); !ok {
		t.Fatalf("unexpected token format: %q", token)
	}
}

func TestGenerateAdvertiserAPITokenIsRandom(t *testing.T) {
	first, err := generateAdvertiserAPIToken()
	if err != nil {
		t.Fatalf("generate first token: %v", err)
	}
	second, err := generateAdvertiserAPIToken()
	if err != nil {
		t.Fatalf("generate second token: %v", err)
	}
	if first == second {
		t.Fatalf("two generated tokens unexpectedly match")
	}
}
