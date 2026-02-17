#!/usr/bin/env python3
from __future__ import annotations

from pathlib import Path
from PIL import Image, ImageDraw, ImageFont


ROOT = Path(__file__).resolve().parent.parent
OUT_DIR = ROOT / "docs" / "launch"
W, H = 1536, 1024

BG = "#0B1220"
SURFACE = "#111B30"
SURFACE_ALT = "#0F172A"
BORDER = "#1F2A44"
TEXT = "#E5E7EB"
MUTED = "#94A3B8"
GREEN = "#16A34A"
RED = "#EF4444"
BLUE = "#38BDF8"
AMBER = "#F59E0B"


def font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont | ImageFont.ImageFont:
    candidates = [
        "/System/Library/Fonts/Supplemental/SF Pro Text.ttf",
        "/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
        "/Library/Fonts/Arial.ttf",
    ]
    for path in candidates:
        p = Path(path)
        if p.exists():
            try:
                return ImageFont.truetype(str(p), size=size)
            except Exception:
                pass
    return ImageFont.load_default()


F_TITLE = font(58, bold=True)
F_H2 = font(36, bold=True)
F_H3 = font(28, bold=True)
F_BODY = font(24)
F_CAP = font(20)


def panel(draw: ImageDraw.ImageDraw, x: int, y: int, w: int, h: int, title: str | None = None):
    draw.rounded_rectangle((x, y, x + w, y + h), radius=20, fill=SURFACE, outline=BORDER, width=2)
    if title:
        draw.text((x + 20, y + 14), title, fill=TEXT, font=F_H3)


def bullet(draw: ImageDraw.ImageDraw, x: int, y: int, text: str, color: str = GREEN):
    draw.ellipse((x, y + 8, x + 10, y + 18), fill=color)
    draw.text((x + 18, y), text, fill=TEXT, font=F_BODY)


def arrow(draw: ImageDraw.ImageDraw, x1: int, y1: int, x2: int, y2: int, color: str = BLUE):
    draw.line((x1, y1, x2, y2), fill=color, width=4)
    if x2 >= x1:
        tip = [(x2, y2), (x2 - 14, y2 - 8), (x2 - 14, y2 + 8)]
    else:
        tip = [(x2, y2), (x2 + 14, y2 - 8), (x2 + 14, y2 + 8)]
    draw.polygon(tip, fill=color)


def render_architecture():
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    d.text((56, 40), "Tokfence Architecture", fill=TEXT, font=F_TITLE)
    d.text((56, 112), "Agents-first desktop + UDS/TCP dual transport + ImmuneFence", fill=MUTED, font=F_CAP)

    panel(d, 56, 180, 280, 260, "Clients")
    bullet(d, 82, 242, "OpenClaw")
    bullet(d, 82, 286, "CLI")
    bullet(d, 82, 330, "Desktop App")

    panel(d, 390, 180, 280, 260, "Transport")
    bullet(d, 416, 242, "UDS: ~/.tokfence/tokfence.sock", BLUE)
    bullet(d, 416, 286, "TCP: 127.0.0.1:9471", BLUE)
    bullet(d, 416, 330, "Socket perms: 0660", BLUE)

    panel(d, 724, 180, 360, 260, "Tokfence Core")
    bullet(d, 750, 242, "Proxy routing + header sanitization")
    bullet(d, 750, 286, "Budget + rate limits + kill switch")
    bullet(d, 750, 330, "Live logs + stats + watch")

    panel(d, 1138, 180, 340, 260, "Providers")
    bullet(d, 1164, 242, "Anthropic / OpenAI / Groq")
    bullet(d, 1164, 286, "Mistral / Google / OpenRouter")
    bullet(d, 1164, 330, "Custom upstreams")

    arrow(d, 336, 310, 390, 310)
    arrow(d, 670, 310, 724, 310)
    arrow(d, 1084, 310, 1138, 310)

    panel(d, 56, 500, 700, 440, "Security Layer (ImmuneFence)")
    bullet(d, 84, 562, "Capability token validation (Ed25519)")
    bullet(d, 84, 606, "Risk ladder: GREEN -> YELLOW -> ORANGE -> RED", AMBER)
    bullet(d, 84, 650, "Sensor scans: secret refs, endpoint abuse")
    bullet(d, 84, 694, "Canary leak tripwire -> RED escalation", RED)
    bullet(d, 84, 738, "Scope restrictions per risk state")

    panel(d, 800, 500, 678, 440, "Data Plane Guarantees")
    bullet(d, 828, 562, "Keys stay in vault (Keychain/Argon2 file)")
    bullet(d, 828, 606, "Agent receives dummy apiKey only")
    bullet(d, 828, 650, "Upstream auth injected at request time")
    bullet(d, 828, 694, "No secret bodies in normal request logs")
    bullet(d, 828, 738, "Desktop snapshots: local-only, 0600")

    img.save(OUT_DIR / "infographic_architecture.png", format="PNG")


def render_security_model():
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    d.text((56, 40), "Tokfence Security Model", fill=TEXT, font=F_TITLE)
    d.text((56, 112), "Practical trust boundaries and layered controls", fill=MUTED, font=F_CAP)

    panel(d, 56, 180, 460, 760, "Threats")
    bullet(d, 84, 250, "Prompt injection attempts to exfiltrate keys", RED)
    bullet(d, 84, 300, "Compromised local process calls proxy", RED)
    bullet(d, 84, 350, "Runaway token spend or abuse", RED)
    bullet(d, 84, 400, "Dangerous endpoint/method misuse", RED)
    bullet(d, 84, 450, "Leaked token appears in outputs", RED)
    d.text((84, 530), "Boundary note:", fill=TEXT, font=F_H3)
    d.text((84, 574), "localhost is not a full isolation boundary.", fill=MUTED, font=F_BODY)
    d.text((84, 612), "UDS + capabilities reduce attack surface", fill=MUTED, font=F_BODY)
    d.text((84, 650), "but do not replace host compromise defenses.", fill=MUTED, font=F_BODY)

    panel(d, 556, 180, 924, 760, "Controls")
    bullet(d, 584, 250, "Vault-backed key injection (no plaintext key in agent)")
    bullet(d, 584, 300, "Dual listener: UDS preferred, TCP for Docker flows", BLUE)
    bullet(d, 584, 350, "Capability gating before upstream/tool actions")
    bullet(d, 584, 400, "Risk-state policy enforcement (scope + TTL)")
    bullet(d, 584, 450, "Budget/rate-limit/kill switch containment")
    bullet(d, 584, 500, "Canary leak detection with incident logging", AMBER)

    risk_x = 620
    risk_y = 590
    risk_w = 780
    risk_h = 290
    d.rounded_rectangle((risk_x, risk_y, risk_x + risk_w, risk_y + risk_h), radius=16, fill=SURFACE_ALT, outline=BORDER, width=2)
    d.text((risk_x + 24, risk_y + 16), "Risk Escalation", fill=TEXT, font=F_H3)

    segments = [
        ("GREEN", GREEN),
        ("YELLOW", AMBER),
        ("ORANGE", "#FB923C"),
        ("RED", RED),
    ]
    sx = risk_x + 28
    sy = risk_y + 86
    sw = 170
    for name, color in segments:
        d.rounded_rectangle((sx, sy, sx + sw, sy + 76), radius=12, fill=color, outline=None)
        d.text((sx + 46, sy + 24), name, fill="#0B1220", font=F_H3)
        if sx + sw + 42 < risk_x + risk_w:
            arrow(d, sx + sw + 8, sy + 38, sx + sw + 34, sy + 38, color=BLUE)
        sx += sw + 50

    d.text((risk_x + 24, risk_y + 196), "No auto-downgrade in-session. New session/reset required.", fill=MUTED, font=F_BODY)

    img.save(OUT_DIR / "infographic_security_model.png", format="PNG")


def render_control_plane():
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    d.text((56, 40), "Tokfence Control Plane", fill=TEXT, font=F_TITLE)
    d.text((56, 112), "Desktop + CLI operate the same local daemon state", fill=MUTED, font=F_CAP)

    panel(d, 56, 180, 720, 760, "Operator Surfaces")
    bullet(d, 84, 250, "Agents tab: guided setup and runtime controls")
    bullet(d, 84, 300, "Overview / Activity / Budget / Providers")
    bullet(d, 84, 350, "Desktop snapshots for widget + menu bar")
    bullet(d, 84, 400, "CLI JSON contract for desktop integration")

    d.text((84, 490), "Primary actions", fill=TEXT, font=F_H3)
    actions = [
        "tokfence start / stop / status",
        "tokfence launch / restart / logs",
        "tokfence kill / unkill / revoke / restore",
        "tokfence budget set / clear / status",
        "tokfence watch --once / --interval",
    ]
    y = 540
    for line in actions:
        d.text((84, y), f"- {line}", fill=MUTED, font=F_BODY)
        y += 44

    panel(d, 816, 180, 664, 760, "Runtime State")
    state_lines = [
        ("Daemon", "Online", GREEN),
        ("Transport", "UDS preferred + TCP fallback", BLUE),
        ("Capabilities", "Session scoped + signed", GREEN),
        ("Risk", "GREEN / YELLOW / ORANGE / RED", AMBER),
        ("Kill switch", "Immediate global block", RED),
        ("Audit", "SQLite request logs + stats", GREEN),
    ]
    y = 250
    for key, val, col in state_lines:
        d.rounded_rectangle((844, y, 1452, y + 90), radius=14, fill=SURFACE_ALT, outline=BORDER, width=2)
        d.text((870, y + 20), key, fill=TEXT, font=F_H3)
        d.text((1110, y + 24), val, fill=col, font=F_BODY)
        y += 104

    img.save(OUT_DIR / "infographic_control_plane.png", format="PNG")


def render_leak_detection():
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    d.text((56, 40), "Key Leak Detector", fill=TEXT, font=F_TITLE)
    d.text((56, 112), "Compare provider usage vs Tokfence logs and raise incidents", fill=MUTED, font=F_CAP)

    panel(d, 56, 180, 1424, 260, "Detection Inputs")
    bullet(d, 84, 248, "Local logs: request count, tokens, cost by provider")
    bullet(d, 84, 292, "Remote usage endpoint: billing/usage totals")
    bullet(d, 84, 336, "Thresholds: absolute delta + percentage + idle-window")

    panel(d, 56, 470, 1424, 470, "Detection Timeline")
    chart_x, chart_y, chart_w, chart_h = 92, 560, 1360, 300
    d.rounded_rectangle((chart_x, chart_y, chart_x + chart_w, chart_y + chart_h), radius=12, fill=SURFACE_ALT, outline=BORDER, width=2)

    points_local = [(0.05, 0.78), (0.20, 0.70), (0.35, 0.58), (0.50, 0.54), (0.65, 0.48), (0.80, 0.44), (0.93, 0.40)]
    points_remote = [(0.05, 0.78), (0.20, 0.70), (0.35, 0.58), (0.50, 0.51), (0.65, 0.40), (0.80, 0.30), (0.93, 0.18)]

    def to_xy(points):
        out = []
        for px, py in points:
            out.append((chart_x + int(px * chart_w), chart_y + int(py * chart_h)))
        return out

    local_xy = to_xy(points_local)
    remote_xy = to_xy(points_remote)
    d.line(local_xy, fill=GREEN, width=5)
    d.line(remote_xy, fill=RED, width=5)

    for p in local_xy:
        d.ellipse((p[0] - 5, p[1] - 5, p[0] + 5, p[1] + 5), fill=GREEN)
    for p in remote_xy:
        d.ellipse((p[0] - 5, p[1] - 5, p[0] + 5, p[1] + 5), fill=RED)

    d.text((chart_x + 30, chart_y + 18), "Green = Tokfence local usage", fill=GREEN, font=F_CAP)
    d.text((chart_x + 30, chart_y + 52), "Red = provider-side usage", fill=RED, font=F_CAP)

    ax = chart_x + int(0.82 * chart_w)
    ay = chart_y + int(0.30 * chart_h)
    d.rounded_rectangle((ax - 170, ay - 120, ax + 220, ay + 30), radius=12, fill="#2B0E15", outline=RED, width=2)
    d.text((ax - 150, ay - 94), "Delta exceeds threshold", fill=TEXT, font=F_BODY)
    d.text((ax - 150, ay - 54), "Risk -> RED, optional auto-revoke", fill=TEXT, font=F_CAP)
    arrow(d, ax - 30, ay - 8, ax + 10, ay + 18, color=RED)

    img.save(OUT_DIR / "infographic_leak_detection.png", format="PNG")


def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    render_architecture()
    render_security_model()
    render_control_plane()
    render_leak_detection()
    print(f"updated infographics in {OUT_DIR}")


if __name__ == "__main__":
    main()
