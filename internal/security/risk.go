package security

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

type RiskState string

const (
	RiskGreen  RiskState = "GREEN"
	RiskYellow RiskState = "YELLOW"
	RiskOrange RiskState = "ORANGE"
	RiskRed    RiskState = "RED"
)

type RiskEvent string

const (
	RiskEventSecretLeak RiskEvent = "secret_leak"
	RiskEventOverride   RiskEvent = "system_override"
	RiskEventEndpoint   RiskEvent = "disallowed_endpoint"
	RiskEventCanaryLeak RiskEvent = "canary_leak"
)

const (
	ScopeProxyAll  = "proxy"
	ScopeProxySafe = "safe"
)

var safeRoutePrefixes = []string{
	"/v1/models",
	"/v1/models/",
	"/models",
	"/models/",
}

type RiskDefaults struct {
	InitialState string
}

type RiskMachine struct {
	mu         sync.RWMutex
	state      RiskState
	seenEvents []RiskEvent
	sessions   map[string]RiskMachineSession
}

type RiskMachineSession struct {
	state      RiskState
	seenEvents []RiskEvent
}

func NewRiskMachine(defaults RiskDefaults) *RiskMachine {
	state, _ := ParseRiskState(defaults.InitialState)
	return &RiskMachine{
		state:      state,
		seenEvents: []RiskEvent{},
		sessions:   map[string]RiskMachineSession{},
	}
}

func (r *RiskMachine) State() RiskState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.state
}

func (r *RiskMachine) StateForSession(sessionID string) RiskState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sessionState, ok := r.sessions[normalizeSessionID(sessionID)]
	if !ok {
		return r.state
	}
	return sessionState.state
}

func (r *RiskMachine) Events() []RiskEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	copyEvents := make([]RiskEvent, len(r.seenEvents))
	copy(copyEvents, r.seenEvents)
	return copyEvents
}

func (r *RiskMachine) EventsForSession(sessionID string) []RiskEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sessionState, ok := r.sessions[normalizeSessionID(sessionID)]
	if !ok {
		return nil
	}
	copyEvents := make([]RiskEvent, len(sessionState.seenEvents))
	copy(copyEvents, sessionState.seenEvents)
	return copyEvents
}

func (r *RiskMachine) Escalate(evt RiskEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seenEvents = append(r.seenEvents, evt)
	next := escalateState(r.state, evt)
	if next == r.state {
		return
	}
	r.state = next
}

func (r *RiskMachine) EscalateForSession(sessionID string, evt RiskEvent) {
	sessionID = normalizeSessionID(sessionID)
	r.mu.Lock()
	defer r.mu.Unlock()

	session := r.sessions[sessionID]
	session.state = r.state
	session.seenEvents = append(session.seenEvents, evt)

	session.state = escalateState(session.state, evt)
	r.sessions[sessionID] = session
}

func (r *RiskMachine) IsRequestAllowedForSession(sessionID string, capabilityScope, method, path string) bool {
	sessionState := r.StateForSession(sessionID)
	return r.IsRequestAllowedForState(sessionState, capabilityScope, method, path)
}

func (r *RiskMachine) IsRequestAllowed(capabilityScope, method, path string) bool {
	return r.IsRequestAllowedForState(r.State(), capabilityScope, method, path)
}

func (r *RiskMachine) IsRequestAllowedForState(state RiskState, capabilityScope, method, path string) bool {
	if capabilityScope == "" {
		capabilityScope = ScopeProxyAll
	}
	capabilityScope = strings.ToLower(strings.TrimSpace(capabilityScope))

	if state == "" {
		state = r.State()
	}

	switch state {
	case RiskGreen:
		return true
	case RiskYellow:
		// Yellow increases scrutiny by preferring safe routes and shorter session TTL.
		if capabilityScope == ScopeProxySafe {
			return IsSafeRoute(method, path)
		}
		return IsSafeRoute(method, path)
	case RiskRed:
		return false
	case RiskOrange:
		if capabilityScope == ScopeProxySafe {
			return IsSafeRoute(method, path)
		}
		// Even proxy scope is constrained while in orange state.
		return IsSafeRoute(method, path)
	default:
		return true
	}
}

func ParseRiskState(raw string) (RiskState, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(RiskGreen):
		return RiskGreen, nil
	case string(RiskYellow):
		return RiskYellow, nil
	case string(RiskOrange):
		return RiskOrange, nil
	case string(RiskRed):
		return RiskRed, nil
	case "":
		return RiskGreen, nil
	default:
		return RiskGreen, errors.New("invalid risk state")
	}
}

func ParseRiskStateMust(raw string) RiskState {
	state, err := ParseRiskState(raw)
	if err != nil {
		return RiskGreen
	}
	return state
}

func ValidateRiskState(raw string) error {
	_, err := ParseRiskState(raw)
	return err
}

func IsSafeRoute(method, path string) bool {
	m := strings.ToUpper(strings.TrimSpace(method))
	p := strings.ToLower(strings.TrimSpace(path))
	if p == "" {
		return false
	}

	switch m {
	case "GET", "HEAD", "OPTIONS":
		// intentionally allow only read-ish methods for safe mode
	default:
		return false
	}
	for _, prefix := range safeRoutePrefixes {
		if strings.HasPrefix(p, strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

func CanUseScope(scope string) bool {
	s := strings.ToLower(strings.TrimSpace(scope))
	if s == "" {
		return false
	}
	for _, allowed := range []string{ScopeProxyAll, ScopeProxySafe} {
		if s == allowed {
			return true
		}
	}
	return false
}

func NormalizeScope(scope string) string {
	s := strings.ToLower(strings.TrimSpace(scope))
	if !CanUseScope(s) {
		return ScopeProxyAll
	}
	return s
}

func IsScopeProgressivelySafe(scope string) bool {
	s := NormalizeScope(scope)
	return s == ScopeProxySafe
}

func OrderedRiskStates() []RiskState {
	return []RiskState{RiskGreen, RiskYellow, RiskOrange, RiskRed}
}

func MaxRisk(states ...RiskState) RiskState {
	position := map[RiskState]int{}
	for idx, state := range OrderedRiskStates() {
		position[state] = idx
	}
	max := RiskGreen
	for _, state := range states {
		if position[state] > position[max] {
			max = state
		}
	}
	return max
}

func normalizeSessionID(sessionID string) string {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "default"
	}
	return sessionID
}

func NormalizeSessionID(sessionID string) string {
	return normalizeSessionID(sessionID)
}

func escalateState(current RiskState, evt RiskEvent) RiskState {
	next := current
	switch evt {
	case RiskEventSecretLeak:
		if next == RiskGreen {
			return RiskYellow
		}
	case RiskEventOverride:
		if next == RiskGreen || next == RiskYellow {
			return RiskOrange
		}
	case RiskEventEndpoint:
		if next == RiskGreen || next == RiskYellow {
			return RiskOrange
		}
	case RiskEventCanaryLeak:
		return RiskRed
	default:
		return next
	}
	return next
}

func TopNEvents(events []RiskEvent, max int) []RiskEvent {
	if max <= 0 {
		return nil
	}
	if len(events) <= max {
		return append([]RiskEvent(nil), events...)
	}
	copyEvents := append([]RiskEvent(nil), events...)
	sort.SliceStable(copyEvents, func(i, j int) bool {
		return copyEvents[i] < copyEvents[j]
	})
	return copyEvents[:max]
}
