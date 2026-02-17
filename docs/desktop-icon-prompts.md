# Tokfence Desktop Icon Prompt Pack

This document defines the icon prompt set for Tokfence Desktop.
It is the canonical source for icon generation and export naming.

## Design Language

Reference style:
- OpenAI icon language (Codex, ChatGPT, API docs)
- Monoline or duotone
- Geometric, not illustrative
- Consistent corner radius (about 2px on a 24px canvas)
- Stroke weight 1.5px on 24px canvas, scaled proportionally
- Monochrome tint system with one accent

Canvas and export:
- Design canvas: `24x24` (or `20x20` / `16x16` / `14x14` as specified)
- Export: SVG for conversion to Xcode-compatible PDF/template assets

Color tokens:
- Accent green: `#16a34a` (secure/active)
- Accent red: `#ef4444` (error/revoked)
- Neutral gray 500: `#6b7280`
- Neutral gray 300: `#d1d5db`

## Sidebar Navigation (24x24, monoline, 1.5px)

### 1) Agents
```text
Minimalist icon: hexagonal shield outline with a small circuit-node pattern inside.
Monoline, 1.5px stroke, 24x24 canvas. Geometric, no fill. OpenAI-style clean.
Conveys: autonomous agents under protection.
```

### 2) Overview
```text
Minimalist icon: dashboard grid, four rounded rectangles arranged 2x2, top-left slightly larger.
Monoline, 1.5px stroke, 24x24 canvas. Geometric, OpenAI-style.
Conveys: system overview at a glance.
```

### 3) Vault
```text
Minimalist icon: keyhole inside a rounded rectangle with a subtle lock arch on top.
Monoline, 1.5px stroke, 24x24 canvas. Geometric, no fill. OpenAI-style.
Conveys: encrypted key storage, not a physical safe.
```

### 4) Activity
```text
Minimalist icon: three horizontal lines of varying length stacked vertically, with a small dot at each right end.
Monoline, 1.5px stroke, 24x24 canvas. Geometric, OpenAI-style.
Conveys: request stream, activity log.
```

### 5) Budget
```text
Minimalist icon: circular progress ring (3/4 filled) with a small currency-neutral tick mark in the center.
Monoline, 1.5px stroke, 24x24 canvas. Geometric, OpenAI-style.
Conveys: spend tracking, usage limit.
```

### 6) Providers
```text
Minimalist icon: three overlapping rounded squares offset diagonally, like stacked API cards.
Monoline, 1.5px stroke, 24x24 canvas. Geometric, OpenAI-style.
Conveys: multiple API providers, plug-in architecture.
```

### 7) Settings
```text
Minimalist icon: single gear with 6 teeth and perfectly circular center hole.
Monoline, 1.5px stroke, 24x24 canvas. Geometric, OpenAI-style.
Conveys: configuration.
```

## Agent Card States (20x20, duotone allowed)

### 8) Agent Stopped
```text
Minimalist icon: rounded square outline with a centered right-pointing play triangle.
Monoline, 1.5px stroke, 20x20. Neutral gray. OpenAI-style.
Conveys: ready to start, currently idle.
```

### 9) Agent Running
```text
Minimalist icon: rounded square outline with two vertical bars (pause symbol) inside.
Left edge has a 2px solid green accent stripe.
Duotone: green accent plus gray outline. 20x20. OpenAI-style.
Conveys: actively running.
```

### 10) Agent Error
```text
Minimalist icon: rounded square outline with exclamation mark centered.
Left edge has a 2px solid red accent stripe.
Duotone: red accent plus gray outline. 20x20. OpenAI-style.
Conveys: error state, needs attention.
```

### 11) Agent Starting
```text
Minimalist icon: circular arc (270 degrees) with open gap at top, suggesting rotation.
Monoline, 1.5px stroke, 20x20. Neutral gray. OpenAI-style.
Conveys: loading, in progress. Animated in UI by rotation.
```

## Guided Setup Step Icons (20x20, duotone)

### 12) Docker
```text
Minimalist icon: container box (rectangular prism outline from slight top angle) with a small whale-fin curve on top.
Monoline, 1.5px stroke, 20x20. OpenAI-style.
Conveys: containerized environment.
```

### 13) Daemon
```text
Minimalist icon: vertical lightning bolt inside a circle outline.
Monoline, 1.5px stroke, 20x20. OpenAI-style.
Conveys: background process, always-on service.
```

### 14) Vault Setup
```text
Minimalist icon: key rotated 45 degrees with a small plus sign at the bow end.
Monoline, 1.5px stroke, 20x20. OpenAI-style.
Conveys: add API key to vault.
```

### 15) Container Launch
```text
Minimalist icon: rocket outline, geometric and minimal.
Only nose cone/body and two small fin lines, no flames.
Monoline, 1.5px stroke, 20x20. OpenAI-style.
Conveys: launch, deploy.
```

### 16) Step Completed
```text
Minimalist icon: circle outline with centered checkmark.
Duotone: green fill circle and white checkmark. 20x20. OpenAI-style.
Conveys: done, verified.
```

### 17) Step Pending
```text
Minimalist icon: dashed circle outline (4px dash, 3px gap).
Monoline, 1.5px stroke, 20x20. Light gray. OpenAI-style.
Conveys: not started, waiting.
```

### 18) Step Active
```text
Minimalist icon: circle outline with centered filled dot (4px diameter).
Monoline, 1.5px stroke, 20x20. Green accent. OpenAI-style.
Conveys: currently in progress.
```

## Action Buttons (16x16, monoline)

### 19) Start Securely
```text
Minimalist icon: play triangle with tiny shield badge at bottom-right.
Monoline, 1.5px stroke, 16x16. Green. OpenAI-style.
Conveys: secure launch action.
```

### 20) Stop
```text
Minimalist icon: rounded square stop symbol.
Monoline, 1.5px stroke, 16x16. Neutral gray. OpenAI-style.
```

### 21) Restart
```text
Minimalist icon: single circular arrow with arrowhead, 300-degree sweep.
Monoline, 1.5px stroke, 16x16. Neutral gray. OpenAI-style.
```

### 22) Open (External Link)
```text
Minimalist icon: square with arrow pointing top-right, exiting square.
Monoline, 1.5px stroke, 16x16. Neutral gray. OpenAI-style.
```

### 23) Retry
```text
Minimalist icon: circular arrow with small exclamation dot in the center.
Monoline, 1.5px stroke, 16x16. Red accent. OpenAI-style.
```

## Menu Bar Status (16x16, template)

Template requirement:
- pure black plus transparency only
- must render correctly on light and dark menu bar

### 24) Tokfence Active
```text
Minimalist icon: shield outline with a small vertical power-on indicator line inside.
Monoline, 1.5px stroke, 16x16. Pure black template.
Conveys: running and protecting.
```

### 25) Tokfence Inactive
```text
Same shield as active with a diagonal slash.
Monoline, 1.5px stroke, 16x16. Pure black template.
Conveys: daemon not running.
```

### 26) Tokfence Alert
```text
Same shield with a small filled dot at top-right (badge position).
Monoline, 1.5px stroke, 16x16. Pure black template.
Conveys: attention required.
```

## Security / ImmuneFence (20x20, duotone)

### 27) Capability Token
```text
Minimalist icon: hexagon outline with fingerprint-like curved pattern (three curved lines).
Monoline, 1.5px stroke, 20x20. OpenAI-style.
Conveys: cryptographic capability, scoped permission.
```

### 28) Risk Low
```text
Minimalist icon: shield outline with one horizontal bar at bottom third.
Duotone: green bar plus gray shield. 20x20. OpenAI-style.
Conveys: low risk.
```

### 29) Risk Elevated
```text
Minimalist icon: shield outline with two horizontal bars in lower area.
Duotone: amber bars plus gray shield. 20x20. OpenAI-style.
Conveys: elevated caution.
```

### 30) Risk Critical
```text
Minimalist icon: shield with three horizontal bars and exclamation overlay.
Duotone: red fill and white exclamation. 20x20. OpenAI-style.
Conveys: active threat.
```

### 31) Canary
```text
Minimalist icon: geometric bird silhouette (five straight lines) inside a diamond outline.
Monoline, 1.5px stroke, 20x20. OpenAI-style.
Conveys: canary token, tripwire detection.
```

### 32) Sensor Scan
```text
Minimalist icon: radar sweep, circle with a radius line and a small arc near tip.
Monoline, 1.5px stroke, 20x20. OpenAI-style.
Conveys: active monitoring.
```

## Provider Chips (14x14, monoline)

### 33) Anthropic
```text
Minimalist capital A in geometric sans-serif, single stroke weight.
14x14, neutral gray.
```

### 34) OpenAI
```text
Minimalist hexagonal spiral/aperture, simplified monoline mark.
14x14, neutral gray.
```

### 35) Google
```text
Minimalist capital G in geometric sans-serif, single stroke.
14x14, neutral gray.
```

### 36) Mistral
```text
Minimalist capital M in geometric sans-serif, single stroke.
14x14, neutral gray.
```

### 37) Generic Provider
```text
Minimalist connector icon: circle with two short prongs at bottom.
14x14, neutral gray.
Conveys: unlisted provider.
```

## Export Notes

- Format: SVG (source), converted to PDF/template assets for Xcode
- Naming convention: `tf-icon-{category}-{name}.svg`
- Example names:
  - `tf-icon-nav-agents.svg`
  - `tf-icon-nav-vault.svg`
  - `tf-icon-state-running.svg`
  - `tf-icon-setup-docker.svg`
  - `tf-icon-action-start.svg`
  - `tf-icon-menubar-active.svg`
  - `tf-icon-security-risk-low.svg`
  - `tf-icon-provider-anthropic.svg`
- Size variants: `@1x` and `@2x` at each native canvas size (`24`, `20`, `16`, `14`)
- Menu bar icons must be exported as template assets (black plus alpha only)
