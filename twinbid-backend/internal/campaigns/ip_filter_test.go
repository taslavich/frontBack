package campaigns

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"twinbid-backend/internal/httpx"
	"twinbid-backend/internal/models"
)

func TestNormalizeAndValidateIPv4TargetingFilter(t *testing.T) {
	filter := models.TargetingFilter{
		IsWhiteList: false,
		Objects: []string{
			"1.2.3.4",
			"192.168.1.123/24",
			"10.0.0.0/8",
			" 1.2.3.4 ",
		},
	}

	if err := normalizeAndValidateIPv4TargetingFilter(&filter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"1.2.3.4", "192.168.1.0/24", "10.0.0.0/8"}
	if !reflect.DeepEqual(filter.Objects, want) {
		t.Fatalf("objects=%v want=%v", filter.Objects, want)
	}
}

func TestNormalizeAndValidateIPv4TargetingFilterRejectsEveryInvalidKind(t *testing.T) {
	original := []string{"999.10.1.1", "192.168.1.0/35", "abc", "10.0.0", "2001:db8::1", "2001:db8::/32", ""}
	filter := models.TargetingFilter{IsWhiteList: true, Objects: append([]string(nil), original...)}

	err := normalizeAndValidateIPv4TargetingFilter(&filter)
	if err == nil {
		t.Fatal("expected validation error")
	}
	var httpErr httpx.HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("error type=%T want httpx.HTTPError", err)
	}
	if httpErr.Status != http.StatusBadRequest {
		t.Fatalf("status=%d want=%d", httpErr.Status, http.StatusBadRequest)
	}
	for _, value := range original[:6] {
		if !strings.Contains(httpErr.Message, value) {
			t.Fatalf("error %q does not mention invalid value %q", httpErr.Message, value)
		}
	}
	if !reflect.DeepEqual(filter.Objects, original) {
		t.Fatalf("invalid filter was partially mutated: got=%v want=%v", filter.Objects, original)
	}
}

func TestNormalizeIPv4CIDRZeroPrefix(t *testing.T) {
	got, err := normalizeIPv4OrCIDR("192.168.1.123/0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "0.0.0.0/0" {
		t.Fatalf("got=%q want=%q", got, "0.0.0.0/0")
	}
}
