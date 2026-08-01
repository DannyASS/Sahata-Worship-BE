package livekit

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrNotConfigured = errors.New("livekit belum dikonfigurasi")

const tokenTTL = 90 * time.Minute

type Config struct {
	WebSocketURL string
	APIKey       string
	APISecret    string
}

type TokenRequest struct {
	RoomID     int64
	Identity   string
	Name       string
	Role       string
	CanPublish bool
}

type videoGrant struct {
	Room              string   `json:"room"`
	RoomJoin          bool     `json:"roomJoin"`
	CanPublish        bool     `json:"canPublish"`
	CanSubscribe      bool     `json:"canSubscribe"`
	CanPublishData    bool     `json:"canPublishData"`
	CanPublishSources []string `json:"canPublishSources,omitempty"`
}

type claims struct {
	jwt.RegisteredClaims
	Name     string     `json:"name,omitempty"`
	Metadata string     `json:"metadata,omitempty"`
	Video    videoGrant `json:"video"`
}

type Issuer struct{ config Config }

func NewIssuer(config Config) *Issuer { return &Issuer{config: config} }

func (i *Issuer) Connection(req TokenRequest) (string, string, error) {
	if strings.TrimSpace(i.config.WebSocketURL) == "" || strings.TrimSpace(i.config.APIKey) == "" || strings.TrimSpace(i.config.APISecret) == "" {
		return "", "", ErrNotConfigured
	}
	if req.RoomID < 1 || strings.TrimSpace(req.Identity) == "" {
		return "", "", errors.New("identitas LiveKit tidak valid")
	}

	now := time.Now()
	grant := videoGrant{Room: "church-worship-" + strconv.FormatInt(req.RoomID, 10), RoomJoin: true, CanPublish: req.CanPublish, CanSubscribe: true, CanPublishData: false}
	if req.CanPublish {
		grant.CanPublishSources = []string{"microphone"}
	}
	metadata, err := json.Marshal(map[string]string{"role": req.Role})
	if err != nil {
		return "", "", err
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		RegisteredClaims: jwt.RegisteredClaims{Issuer: i.config.APIKey, Subject: req.Identity, NotBefore: jwt.NewNumericDate(now.Add(-5 * time.Second)), ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)), IssuedAt: jwt.NewNumericDate(now)},
		Name:             req.Name, Metadata: string(metadata), Video: grant,
	}).SignedString([]byte(i.config.APISecret))
	if err != nil {
		return "", "", err
	}
	return i.config.WebSocketURL, raw, nil
}
