package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	capabilityTokenSeparator = "."
	maxNonceLen              = 12
)

// Capability defines the per-request authorization token used by the daemon.
type Capability struct {
	ClientID  string `json:"client_id"`
	SessionID string `json:"session_id"`
	Scope     string `json:"scope"`
	RiskState string `json:"risk_state"`
	Expiry    int64  `json:"expiry"`
	Nonce     string `json:"nonce"`
	IssuedAt  int64  `json:"issued_at,omitempty"`
}

type CapabilityManager struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
}

type capabilityPayload struct {
	ClientID  string `json:"client_id"`
	SessionID string `json:"session_id"`
	Scope     string `json:"scope"`
	RiskState string `json:"risk_state"`
	Expiry    int64  `json:"expiry"`
	Nonce     string `json:"nonce"`
	IssuedAt  int64  `json:"issued_at,omitempty"`
}

var (
	defaultManager     *CapabilityManager
	defaultManagerOnce sync.Once
	defaultManagerErr  error
)

func defaultCapabilityManager() (*CapabilityManager, error) {
	defaultManagerOnce.Do(func() {
		defaultManager, defaultManagerErr = NewCapabilityManager()
	})
	return defaultManager, defaultManagerErr
}

func NewCapabilityManager() (*CapabilityManager, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	return &CapabilityManager{
		privateKey: private,
		publicKey:  public,
	}, nil
}

func MintCapability(scope, clientID, sessionID, riskState string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", errors.New("client id is required")
	}
	manager, err := defaultCapabilityManager()
	if err != nil {
		return "", err
	}
	return manager.mintCapability(scope, clientID, sessionID, riskState, ttl)
}

func MintCapabilityForRisk(scope, clientID, sessionID, riskState string) (string, error) {
	return MintCapability(scope, clientID, sessionID, riskState, 12*time.Minute)
}

func ValidateCapability(token string) (Capability, error) {
	manager, err := defaultCapabilityManager()
	if err != nil {
		return Capability{}, err
	}
	return manager.ValidateCapability(token)
}

func NewCapabilityManagerFromKey(private ed25519.PrivateKey, public ed25519.PublicKey) (*CapabilityManager, error) {
	if len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key size")
	}
	if len(public) != ed25519.PublicKeySize {
		return nil, errors.New("invalid public key size")
	}
	return &CapabilityManager{privateKey: private, publicKey: public}, nil
}

func (m *CapabilityManager) MintCapability(scope, clientID, sessionID, riskState string, ttl time.Duration) (string, error) {
	if m == nil {
		return "", errors.New("capability manager is nil")
	}
	return m.mintCapability(scope, clientID, sessionID, riskState, ttl)
}

func (m *CapabilityManager) MintCapabilityForRisk(scope, clientID, sessionID, riskState string) (string, error) {
	return m.MintCapability(scope, clientID, sessionID, riskState, 12*time.Minute)
}

func (m *CapabilityManager) mintCapability(scope, clientID, sessionID, riskState string, ttl time.Duration) (string, error) {
	if strings.TrimSpace(clientID) == "" {
		return "", errors.New("client id is required")
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}
	if strings.TrimSpace(scope) == "" {
		scope = ScopeProxyAll
	}
	parsedRisk, err := ParseRiskState(riskState)
	if err != nil {
		parsedRisk = RiskGreen
	}
	ttl = normalizeTTL(ttl)
	if parsedRisk == RiskYellow {
		ttl = halfTTL(ttl)
	}
	payload := Capability{
		ClientID:  strings.TrimSpace(clientID),
		SessionID: strings.TrimSpace(sessionID),
		Scope:     strings.TrimSpace(strings.ToLower(scope)),
		RiskState: string(parsedRisk),
		Expiry:    time.Now().Add(ttl).Unix(),
		IssuedAt:  time.Now().UTC().Unix(),
		Nonce:     randomHex(maxNonceLen),
	}
	encoded, err := json.Marshal(capabilityPayload(payload))
	if err != nil {
		return "", fmt.Errorf("encode capability: %w", err)
	}
	sig := ed25519.Sign(m.privateKey, encoded)
	return base64.RawURLEncoding.EncodeToString(encoded) + capabilityTokenSeparator + base64.RawURLEncoding.EncodeToString(sig), nil
}

func (m *CapabilityManager) ValidateCapability(token string) (Capability, error) {
	if m == nil {
		return Capability{}, errors.New("capability manager is nil")
	}
	parts := strings.Split(token, capabilityTokenSeparator)
	if len(parts) != 2 {
		return Capability{}, errors.New("invalid capability format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Capability{}, fmt.Errorf("decode payload: %w", err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Capability{}, fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(m.publicKey, payload, sig) {
		return Capability{}, errors.New("invalid capability signature")
	}

	var c Capability
	if err := json.Unmarshal(payload, &c); err != nil {
		return Capability{}, fmt.Errorf("parse capability: %w", err)
	}
	if err := ValidateRiskState(c.RiskState); err != nil {
		return Capability{}, err
	}
	if c.ClientID == "" || c.SessionID == "" || c.Nonce == "" {
		return Capability{}, errors.New("invalid capability payload")
	}
	if strings.TrimSpace(c.Scope) == "" {
		c.Scope = ScopeProxyAll
	}
	if c.Expiry <= time.Now().Unix() {
		return Capability{}, errors.New("capability expired")
	}
	return c, nil
}

func randomHex(size int) string {
	if size <= 0 {
		size = maxNonceLen
	}
	buf := make([]byte, size)
	_, _ = rand.Read(buf)
	const alphabet = "0123456789abcdef"
	for i := range buf {
		buf[i] = alphabet[int(buf[i])%len(alphabet)]
	}
	return string(buf)
}

func normalizeTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 12 * time.Minute
	}
	if ttl < time.Second {
		return time.Second
	}
	return ttl
}

func halfTTL(ttl time.Duration) time.Duration {
	ttl = normalizeTTL(ttl)
	half := ttl / 2
	if half < time.Second {
		return time.Second
	}
	return half
}

func formatTTL(ttl time.Duration) string {
	if ttl <= 0 {
		return "0s"
	}
	return strconv.FormatFloat(ttl.Seconds(), 'f', -1, 64) + "s"
}
