package webhook

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Header names for webhook security.
const (
	HeaderSignature = "X-JobBoard-Signature"
	HeaderTimestamp = "X-JobBoard-Timestamp"
)

// GenerateSecret generates a cryptographically secure random secret token with a 'whsec_' prefix.
func GenerateSecret() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random secret: %w", err)
	}
	return "whsec_" + hex.EncodeToString(bytes), nil
}

// ComputeSignature calculates the HMAC-SHA256 signature for a webhook payload.
// Format: sha256=HMAC_SHA256(secret, "t=${timestamp}.${rawBody}")
func ComputeSignature(secret string, timestamp int64, rawBody []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("t=%d.", timestamp)))
	mac.Write(rawBody)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature validates a webhook payload signature and checks timestamp tolerance to prevent replay attacks.
func VerifySignature(secret string, timestamp int64, rawBody []byte, signature string, tolerance time.Duration) bool {
	if tolerance > 0 {
		now := time.Now().Unix()
		diff := now - timestamp
		if diff < 0 {
			diff = -diff
		}
		if time.Duration(diff)*time.Second > tolerance {
			return false
		}
	}

	expectedSig := ComputeSignature(secret, timestamp, rawBody)
	return hmac.Equal([]byte(expectedSig), []byte(signature))
}
