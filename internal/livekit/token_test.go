package livekit

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueRequiresConfiguration(t *testing.T) {
	issuer := NewIssuer(Config{})
	_, _, err := issuer.Connection(TokenRequest{RoomID: 1, Identity: "user-1"})
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Issue() error = %v, want %v", err, ErrNotConfigured)
	}
}

func TestIssuePublisherToken(t *testing.T) {
	const secret = "test-secret"
	issuer := NewIssuer(Config{WebSocketURL: "wss://livekit.example.com", APIKey: "test-key", APISecret: secret})

	url, token, err := issuer.Connection(TokenRequest{
		RoomID:     42,
		Identity:   "user-7",
		Name:       "Director",
		Role:       "Music Director",
		CanPublish: true,
	})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if url != "wss://livekit.example.com" {
		t.Fatalf("URL = %q", url)
	}

	parsed := &claims{}
	_, err = jwt.ParseWithClaims(token, parsed, func(token *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		t.Fatalf("ParseWithClaims() error = %v", err)
	}
	if parsed.Issuer != "test-key" || parsed.Subject != "user-7" {
		t.Fatalf("unexpected identity claims: issuer=%q subject=%q", parsed.Issuer, parsed.Subject)
	}
	if got := parsed.ExpiresAt.Time.Sub(parsed.IssuedAt.Time); got != 90*time.Minute {
		t.Fatalf("token TTL = %v, want %v", got, 90*time.Minute)
	}
	if !parsed.Video.RoomJoin || parsed.Video.Room != "church-worship-42" || !parsed.Video.CanPublish || !parsed.Video.CanSubscribe {
		t.Fatalf("unexpected publisher grant: %+v", parsed.Video)
	}
	if len(parsed.Video.CanPublishSources) != 1 || parsed.Video.CanPublishSources[0] != "microphone" {
		t.Fatalf("publish sources = %#v", parsed.Video.CanPublishSources)
	}
}

func TestIssueSubscriberToken(t *testing.T) {
	issuer := NewIssuer(Config{WebSocketURL: "wss://livekit.example.com", APIKey: "test-key", APISecret: "test-secret"})
	_, token, err := issuer.Connection(TokenRequest{RoomID: 42, Identity: "member-9", CanPublish: false})
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}

	parsed := &claims{}
	_, err = jwt.ParseWithClaims(token, parsed, func(token *jwt.Token) (any, error) {
		return []byte("test-secret"), nil
	})
	if err != nil {
		t.Fatalf("ParseWithClaims() error = %v", err)
	}
	if parsed.Video.CanPublish || !parsed.Video.CanSubscribe || parsed.Video.CanPublishData {
		t.Fatalf("unexpected subscriber grant: %+v", parsed.Video)
	}
}
