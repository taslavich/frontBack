package campaigns

import (
	"testing"

	"twinbid-backend/internal/models"
)

func TestValidateOSVersionTargeting(t *testing.T) {
	os := models.TargetingFilter{IsWhiteList: true, Objects: []string{"iOS", "Android"}}
	for _, tc := range []struct {
		name    string
		version models.TargetingFilter
		wantErr bool
	}{
		{name: "empty", version: models.TargetingFilter{IsWhiteList: true}},
		{name: "ios major", version: models.TargetingFilter{IsWhiteList: true, Objects: []string{"iOS 18"}}},
		{name: "ios minor", version: models.TargetingFilter{IsWhiteList: false, Objects: []string{"iOS 18.7"}}},
		{name: "android minor", version: models.TargetingFilter{IsWhiteList: true, Objects: []string{"Android 8.1"}}},
		{name: "android 12L", version: models.TargetingFilter{IsWhiteList: true, Objects: []string{"Android 12L"}}},
		{name: "patch not selectable", version: models.TargetingFilter{IsWhiteList: true, Objects: []string{"iOS 18.7.1"}}, wantErr: true},
		{name: "bad os", version: models.TargetingFilter{IsWhiteList: true, Objects: []string{"Windows 11"}}, wantErr: true},
		{name: "bad android suffix", version: models.TargetingFilter{IsWhiteList: true, Objects: []string{"Android 8Go"}}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOSVersionTargeting(os, tc.version)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateOSVersionTargeting() err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestValidateOSVersionTargetingRequiresMatchingOSWhitelist(t *testing.T) {
	version := models.TargetingFilter{IsWhiteList: true, Objects: []string{"Android 12"}}
	if err := validateOSVersionTargeting(models.TargetingFilter{IsWhiteList: true, Objects: []string{"iOS"}}, version); err == nil {
		t.Fatal("expected Android version to require Android in OS whitelist")
	}
	if err := validateOSVersionTargeting(models.TargetingFilter{IsWhiteList: false, Objects: []string{"Windows"}}, version); err == nil {
		t.Fatal("expected os_version to reject non-whitelist OS targeting")
	}
}
