# Launch Assets

This folder contains the screenshots and launch visuals used in README, social posts, and discussions.
Last updated: `2026-02-17`.

## Core screenshots

- `screenshot_agents_guided_missing.png` — Agents view, Guided Setup with unmet prerequisites.
- `screenshot_agents_guided_ready.png` — Guided Setup complete, ready-to-start state.
- `screenshot_agents_transition_runtime.png` — Running state with controls and recent activity.

## Infographics

- `infographic_architecture.png` — Agents/Desktop/CLI -> UDS/TCP -> Tokfence -> provider flow.
- `infographic_security_model.png` — threat boundary and ImmuneFence control layers.
- `infographic_control_plane.png` — operational controls (launch, kill, budget, watch).
- `infographic_leak_detection.png` — local-vs-remote usage delta detection model.
- `social_preview.png` — 1280x640 Open Graph banner for GitHub social preview.

## Notes

- Paths are stable by design so links do not break.
- Files are overwritten in place when assets are updated.
- Regenerate infographics with:
  - `./scripts/generate_launch_infographics.py`
