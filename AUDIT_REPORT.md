# Tokfence Security Audit Report

Date: 2026-02-18  
Auditor: Codex

## Executive Summary

Der Review der genannten Kerndateien (`internal/daemon/server.go`, `internal/proxy/headers.go`, `internal/config/config.go`, `cmd/tokfence/main.go`) zeigt ein stabiles Grundgerüst mit durchgängiger UDS/TCP-Dual-Transport-Architektur, Capability-basierter Request-Gating-Logik und Sensor-Routing in den Proxypfad.  

Es gibt jedoch mehrere Sicherheitslücken auf Policy-Ebene: die strikte Durchsetzung der Capability-Grenzen ist in der Default-Konfiguration abgeschwächt, der Header-Sanitizer arbeitet über Substrings statt expliziter Header-Normalisierung, und der Upstream-HTTP-Client nutzt die Default-Weiterleitungslogik ohne harten Redirect-Hardening.  

Die kritischste Abweichung ist, dass ein kompromittierter lokaler Prozess in einem typischen Setup weiterhin Requests ausführen kann, wenn keine explizite, nicht-kompatible Capability-Verwendung erzwungen wird (Design-Tradeoff im Code).

## Critical Findings

### SEC-01 — Silent Capability auto-issue on missing token bypasses strict policy intent
**Severity:** High  
**File/line range:** `internal/daemon/server.go:239-277`, `internal/config/config.go:87-109`  

- **Issue:** If no `X-Tokfence-Capability` header is present, `validateOrIssueCapability()` synthesizes an internal token per request whenever ImmuneFence is enabled (default path). This weakens strict per-client enforcement and makes “capability-gated” semantics optional-by-default.
- **Why high:** Local attackers with socket/TCP access can send proxied requests without possessing a client-issued token, so the security model depends on token presence being advisory rather than mandatory.
- **Reproduction / exploit snippet:**

```bash
curl -X POST http://127.0.0.1:9471/openai/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}'
```

If the daemon is running and provider key exists, this succeeds because `validateOrIssueCapability()` executes the backward compatibility branch (`header == ""`) and returns a minted token.
- **Current test coverage:** Partial  
  - `internal/daemon/server_test.go` exercises proxy flows that implicitly pass without header in multiple tests (`TestHandleProxyForwardsInjectsAuthAndLogs`, `TestHandleProxyStreamingPassthroughFlushesAndLogsUsage`), but there is no explicit strict-mode test asserting rejection when token is missing in an enforcement scenario.
- **Docs mismatch:**  
  - `README.md` describes ImmuneFence as capability-gated request handling, but does not surface that default local behavior auto-issues capabilities unless configured otherwise.  
  - `SECURITY.md` partially documents “non-strict local mode” issuance, but README’s positioning is less explicit and can overpromise strictness.

### SEC-02 — Substring-based header stripping causes both false positives and potential bypass patterns
**Severity:** Medium  
**File/line range:** `internal/proxy/headers.go:9-39`  

- **Issue:** `StripIncomingAuth()` removes headers and values using `strings.Contains(...)` matching over broad token list (`authorization`, `token`, `api-key`, `bearer`, etc.).  
  - False positives: legitimate non-auth headers containing these substrings are silently removed (e.g., `X-My-Auth-Proxy`, `X-Custom-Token`, arbitrary app-specific headers).  
  - Bypass risk: auth-like headers that do not match substring literals (e.g. atypical naming variants) may escape stripping.
- **Why medium:** It is reliability as well as security-sensitive: incorrect stripping breaks expected behavior and hidden auth-like headers can survive into upstream.
- **Reproduction / exploit snippet:**
  - Stripping side effect: send `-H "X-Custom-Auth: contains bearer token"` and observe removal before upstream forwarding.
  - Potential bypass surface: header names/values using uncommon formats are not normalized into a strict auth-schema matching model and can survive.
- **Current test coverage:** Partial  
  - No dedicated unit test for `StripIncomingAuth` exists in `internal/proxy/headers_test.go` (only `ApplyProviderAuth` tests exist).  
  - Integration tests in `internal/daemon/server_test.go` cover a subset of stripping behavior and include one positive removal case, but not edge patterns/false positives.
- **Docs mismatch:**  
  - `README.md`/`SECURITY.md` describe auth sanitization as strict. The implementation is more heuristic/substring-based than strict canonical header-key matching.

### SEC-03 — Default http.Client follows redirects and can forward injected credentials to attacker hosts
**Severity:** High  
**File/line range:** `internal/daemon/server.go:93`, `internal/daemon/server.go:512`, `internal/daemon/server.go:540-545`  

- **Issue:** Outbound client uses `&http.Client{Transport: transport}` without redirect hardening. Go’s default client follows redirects. Injected provider credentials (e.g., `Authorization` and `x-api-key`) can be forwarded to a second host on redirect in this flow.
- **Why high:** Token/key leakage via upstream 3xx control is a direct credential-exposure risk, and can route secrets to attacker-controlled endpoints that the original request did not target.
- **Reproduction / exploit snippet:**
  - Mock upstream returns `302 Location: https://attacker.example/` on the first response.
  - Observe via attacker listener logs that headers forwarded to redirect target include `x-api-key` and/or `Authorization` (as demonstrated in local `net/http/httptest` check).
- **Current test coverage:** No  
  - No tests validate redirect behavior in daemon request path; no test around header preservation/drop behavior on redirected upstream responses.
- **Docs mismatch:**  
  - `README.md` and `SECURITY.md` do not mention redirect hardening or constraints on upstream hop behavior, leaving an uncommunicated risk in production path.

### SEC-04 — Capability/Risk/Sensor pipeline has policy-depth gaps (notably edge coverage, session semantics, and detector blind spots)
**Severity:** Medium  
**File/line range:** `internal/daemon/server.go:231-293`, `internal/daemon/server.go:296-307`, `internal/daemon/server.go:456-463`, `internal/daemon/server.go:574-576`, with supporting behavior in `internal/security/risk.go:103-129`, `internal/security/sensors.go:10-59`  

- **Issue:** ImmuneFence enforcement exists but lacks hard edges for real-world bypass and abuse cases.
  - Risk escalation and sensor signals run in pipeline, but there is no explicit session-aware token/kill state reset strategy visible in daemon/request path.
  - Sensor detection is regex-based and body-only; encoded/obfuscated payload patterns (`base64`, JSON-escaped key fragments, split markers across chunks) are not covered by current integration path tests.
  - Risk transitions are monotonic and shared per daemon instance, not per session/tenant, creating blast-radius coupling between callers.
- **Why medium:** System is not trivially broken, but threat-detection depth is uneven: detection quality drops on adversarial encoding patterns and multi-tenant/session attack scenarios are under-tested.
- **Reproduction / exploit snippet:**
  - Send payload containing secrets split across JSON encoding/escaping or base64 form. Current regex checks (`DetectSecretReference`) likely miss obfuscation; can observe no escalation where a direct plaintext secret would escalate.
  - Trigger policy checks against one client and observe global state impact across all clients sharing daemon.
- **Current test coverage:** Partial  
  - `internal/security/*_test.go` validates token roundtrip, expiry, tamper checks, and monotonic risk escalation, but daemon-level integration coverage is incomplete.
  - `internal/daemon/server_test.go` has no tests for obfuscation-resistant sensor bypass, session-isolated risk handling, or canary event-to-policy enforcement behavior under concurrency.
- **Docs mismatch:**  
  - Docs communicate a stronger “runtime security hardening layer” effect, but do not explicitly state obfuscation/session-boundary limitations or scope of sensor coverage.

## Summary Table

| Finding ID | File | Severity | One-line description |
|---|---|---|---|
| SEC-01 | `internal/daemon/server.go:239-277` | High | Missing client capability is auto-issued, reducing strict gate enforcement by default. |
| SEC-02 | `internal/proxy/headers.go:9-39` | Medium | Substring-based auth stripping is overly permissive/overbroad and can mis-handle headers. |
| SEC-03 | `internal/daemon/server.go:93,512` | High | Default HTTP client follows redirects and can leak injected credentials to redirect hosts. |
| SEC-04 | `internal/daemon/server.go:231-293,574-576` + security package | Medium | Policy logic is present but edge-case hardening (session boundaries, encoding-aware detection) is incomplete. |

## Recommended Fix Priority

1. **SEC-03 first (High):** Disable/strictly configure redirects (`CheckRedirect` to deny cross-host or strip auth headers before follow), and validate upstream URL allow-list.
2. **SEC-01 (High):** Introduce explicit strict mode and clear CLI/config switch that fails missing/invalid capability on local mode for hardened deployments.
3. **SEC-02 (Medium):** Replace substring stripping with canonical header-name checks and explicit allow/block list; avoid value-based name matching side effects.
4. **SEC-04 (Medium):** Expand risk/sensor policy integration for session-level semantics and adversarial formats; document current guarantees explicitly in docs.

## Missing Test Cases to Add

- `go test` case: **reject missing token in strict mode** (daemon-level; `server_test.go`).
- Unit tests for `StripIncomingAuth`:
  - header-name canonicalization (case/obscure forms),
  - value/Name false-positive matrix,
  - guarantee that non-auth headers containing `token` are not stripped unless intentionally blocked.
- Integration test for redirect leak:
  - upstream 302 to external host,
  - assert sensitive headers are not forwarded.
- Integration test for obfuscated secret/bypass patterns:
  - split-token payloads, base64-in-body, and encoded JSON escapes should either escalate or be flagged by a defense-in-depth sensor strategy.
- Concurrency/session tests for risk machine and capability scope:
  - one client escalating risk should not silently override explicit per-session isolation expectations.
- Config test:
  - socket-path hardening edge cases (owner/permissions + symlink-safe checks) and behavior under invalid socket directory ownership.
