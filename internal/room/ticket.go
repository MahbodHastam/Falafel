package room

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ticketPayload struct {
	Token     string `json:"t"`
	ExpiresAt int64  `json:"e"`
	Nonce     string `json:"n"`
}

// mintTicket creates a short-lived HMAC-signed download ticket.
func mintTicket(secret []byte, token string, ttl time.Duration, now time.Time) (string, time.Time, error) {
	nonce := make([]byte, 9)
	if _, err := rand.Read(nonce); err != nil {
		return "", time.Time{}, fmt.Errorf("generate ticket nonce: %w", err)
	}
	expires := now.Add(ttl)
	payload, err := json.Marshal(ticketPayload{
		Token:     token,
		ExpiresAt: expires.Unix(),
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
	})
	if err != nil {
		return "", time.Time{}, err
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), expires, nil
}

// parseTicket validates a ticket and returns the token it grants.
func parseTicket(secret []byte, value string, now time.Time) (string, error) {
	encoded, signature, ok := strings.Cut(value, ".")
	if !ok || encoded == "" || signature == "" {
		return "", ErrInvalidTicket
	}
	rawSig, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return "", ErrInvalidTicket
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(encoded))
	if !hmac.Equal(rawSig, mac.Sum(nil)) {
		return "", ErrInvalidTicket
	}
	rawPayload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrInvalidTicket
	}
	var payload ticketPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return "", ErrInvalidTicket
	}
	if payload.Token == "" || payload.Nonce == "" {
		return "", ErrInvalidTicket
	}
	if now.Unix() > payload.ExpiresAt {
		return "", ErrInvalidTicket
	}
	return payload.Token, nil
}
