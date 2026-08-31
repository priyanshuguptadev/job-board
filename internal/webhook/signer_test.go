package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/priyanshuguptadev/job-board/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateSecret(t *testing.T) {
	secret1, err := webhook.GenerateSecret()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(secret1, "whsec_"))
	assert.Greater(t, len(secret1), 20)

	secret2, err := webhook.GenerateSecret()
	require.NoError(t, err)
	assert.NotEqual(t, secret1, secret2)
}

func TestComputeAndVerifySignature(t *testing.T) {
	secret := "whsec_test_secret_1234567890"
	rawBody := []byte(`{"id":"evt_1","event":"job.published","data":{"title":"Go Dev"}}`)
	timestamp := time.Now().Unix()

	sig := webhook.ComputeSignature(secret, timestamp, rawBody)
	assert.True(t, strings.HasPrefix(sig, "sha256="))

	// Verify manually
	signedString := fmt.Sprintf("t=%d.%s", timestamp, string(rawBody))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedString))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)

	t.Run("valid signature passes verification", func(t *testing.T) {
		valid := webhook.VerifySignature(secret, timestamp, rawBody, sig, 5*time.Minute)
		assert.True(t, valid)
	})

	t.Run("invalid signature fails verification", func(t *testing.T) {
		valid := webhook.VerifySignature(secret, timestamp, rawBody, "sha256=invalid", 5*time.Minute)
		assert.False(t, valid)
	})

	t.Run("wrong secret fails verification", func(t *testing.T) {
		valid := webhook.VerifySignature("whsec_wrong_secret", timestamp, rawBody, sig, 5*time.Minute)
		assert.False(t, valid)
	})

	t.Run("tampered body fails verification", func(t *testing.T) {
		tamperedBody := []byte(`{"id":"evt_1","event":"job.published","data":{"title":"Tampered"}}`)
		valid := webhook.VerifySignature(secret, timestamp, tamperedBody, sig, 5*time.Minute)
		assert.False(t, valid)
	})

	t.Run("expired timestamp fails tolerance check", func(t *testing.T) {
		oldTimestamp := time.Now().Unix() - 400 // 400 seconds ago > 300s (5min)
		oldSig := webhook.ComputeSignature(secret, oldTimestamp, rawBody)
		valid := webhook.VerifySignature(secret, oldTimestamp, rawBody, oldSig, 5*time.Minute)
		assert.False(t, valid)
	})
}
