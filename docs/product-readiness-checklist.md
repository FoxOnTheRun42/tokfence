# Tokfence Product Readiness Checklist (Production-Grade)

Last updated: 2026-02-17  
Scope: Tokfence as a standalone security product with GA-level quality gates.

Status legend:
- `PASS`: Implemented and verifiable
- `PARTIAL`: Exists, but incomplete or not enforced by default
- `FAIL`: Missing for GA expectations

## GA Blockers (Must Pass Before General Availability)

| # | Requirement | Acceptance Criteria (Pass/Fail) | Current Status | Evidence | Next Action |
| --- | --- | --- | --- | --- | --- |
| 1 | Client authentication beyond localhost | Requests require strong client identity (UDS perms, mTLS, or per-client token) | `FAIL` | `README.md`, `SECURITY.md` explicitly state localhost does not isolate local processes | Implement UDS/mTLS/per-client auth and default-deny unauthenticated clients |
| 2 | Request authorization policy | Enforced allowlists for provider/model/path/method with deny-by-default policy engine | `FAIL` | Current controls are budget/revoke/rate-limit, no model/path policy | Add policy layer in proxy pipeline before upstream call |
| 3 | Log/data hygiene by default | No prompt/response bodies logged by default; sensitive headers redacted; retention + delete controls enforced | `PARTIAL` | Metadata logging + retention documented in `README.md`; no formal data-classification guardrail documented | Add explicit body logging policy + redaction tests + delete workflow |
| 4 | At-rest encryption for all sensitive stores | Vault, DB, and snapshot artifacts encrypted or protected by platform keychain-backed mechanism | `PARTIAL` | Vault encryption documented in `SECURITY.md`; SQLite/snapshot encryption not documented as default | Add optional/required DB encryption mode + snapshot hardening |
| 5 | Release trust chain | Signed/notarized artifacts, SBOM, pinned deps, provenance in CI | `FAIL` | No documented SBOM/signing/notarization pipeline in repo docs | Add release pipeline with signing + SBOM + provenance attestations |
| 6 | Security testing gate | Mandatory fuzz/security/abuse tests + periodic external pentest before release | `PARTIAL` | `go test ./...` exists; no documented fuzzing/pentest gate | Add fuzz targets, smuggling/header abuse tests, and pentest gate in release checklist |
| 7 | Precision of security claims | Marketing/docs must match technical boundary and memory/runtime behavior | `PARTIAL` | Good boundary text exists in `README.md`/`SECURITY.md`, but hero copy can still over-simplify | Tighten top-level claims to “no persistent key storage outside vault” phrasing |

## Public Beta / Serious Use (Should)

| # | Requirement | Acceptance Criteria (Pass/Fail) | Current Status | Evidence | Next Action |
| --- | --- | --- | --- | --- | --- |
| 8 | Scoped proxy tokens | Agents use short-lived scoped Tokfence tokens, revocable independently from provider keys | `FAIL` | No token scope model exposed | Add token mint/rotate/revoke APIs and scope checks |
| 9 | Ops observability | Structured metrics/traces/health endpoints and alerting hooks | `PARTIAL` | CLI stats/logging exists; no OTel/Prometheus in docs | Add metrics endpoint + structured events + alert integration points |
| 10 | Role separation | Distinct admin/operator/agent permissions | `FAIL` | Single-user local model | Add role model and privilege separation defaults |
| 11 | Provider compatibility hardening | Versioned adapters + deterministic streaming/backpressure behavior across providers | `PARTIAL` | Multi-provider support and streaming exist; compatibility contract not formalized | Introduce provider adapter contract tests + version matrix |
| 12 | Hardening guide + secure defaults | Security-first mode documented and defaulted; dev mode explicit | `PARTIAL` | Security model documented; no formal hardening playbook | Add hardening guide with OS/network/process controls |

## Enterprise / Scale (Could, but required for enterprise sales)

| # | Requirement | Acceptance Criteria (Pass/Fail) | Current Status | Evidence | Next Action |
| --- | --- | --- | --- | --- | --- |
| 13 | Team vault + SSO + RBAC | Multi-user authn/authz with auditable role controls | `FAIL` | Not part of current local-first architecture | Add team mode control plane and identity integration |
| 14 | Central policy control plane | Central policy rollout with local enforcement and fleet visibility | `FAIL` | No central management plane | Add policy distribution service and local agent |
| 15 | Compliance package | SOC2/ISO process artifacts, incident workflows, vuln program | `FAIL` | No compliance artifacts in repo | Build secure SDLC evidence and formal vulnerability process |

## Top 3 Immediate Upgrades (Highest Product Credibility Impact)

1. Strong client identity at the proxy boundary (`UDS/mTLS/per-client token`)
2. Log privacy + storage hardening defaults (no body logs + encrypted telemetry stores)
3. Signed release pipeline with SBOM and build provenance

## Suggested GA Gate Rule

GA is allowed only if all `Must` items are `PASS`.  
`Should` items can launch as known gaps only with explicit mitigation notes.  
`Could` items remain roadmap unless enterprise packaging is targeted.

## Mapping to Current Tokfence Claims

- Current docs correctly state the localhost boundary limitation and local-process misuse risk.
- Current docs correctly describe vault cryptography and key-injection model.
- Product messaging should stay aligned with these boundaries until client auth and policy controls are shipped.
