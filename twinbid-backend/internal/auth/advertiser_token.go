package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const advertiserAPITokenPrefix = "adv_"

func generateAdvertiserAPIToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate advertiser API token: %w", err)
	}
	return advertiserAPITokenPrefix + hex.EncodeToString(buf), nil
}
