import SwiftUI

enum TokfenceIconKind {
    case navAgents
    case navOverview
    case navVault
    case navActivity
    case navBudget
    case navProviders
    case navSettings
    case setupDocker
    case setupDaemon
    case setupVault
    case setupContainerLaunch
    case stepComplete
    case stepPending
    case stepActive
    case actionStartSecurely
    case actionStop
    case actionRestart
    case actionOpenExternal
    case actionRetry
    case menuActive
    case menuInactive
    case menuAlert
    case agentStopped
    case agentRunning
    case agentError
    case agentStarting
    case providerAnthropic
    case providerOpenAI
    case providerGoogle
    case providerMistral
    case providerGeneric
}

struct TokfenceIcon: View {
    let kind: TokfenceIconKind
    var size: CGFloat
    var primary: Color
    var accent: Color = TokfenceTheme.accentPrimary
    var secondary: Color = TokfenceTheme.textSecondary

    private var stroke: CGFloat {
        max(1.0, size * 0.0625) // 1.5 at 24pt
    }

    var body: some View {
        ZStack {
            switch kind {
            case .navAgents:
                shieldOutline(color: primary)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.36, y: size * 0.46))
                    p.addLine(to: CGPoint(x: size * 0.50, y: size * 0.40))
                    p.addLine(to: CGPoint(x: size * 0.64, y: size * 0.50))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round, lineJoin: .round))
                Circle().fill(primary).frame(width: size * 0.09, height: size * 0.09).offset(x: -size * 0.14, y: -size * 0.02)
                Circle().fill(primary).frame(width: size * 0.09, height: size * 0.09).offset(x: size * 0.02, y: -size * 0.10)
                Circle().fill(primary).frame(width: size * 0.09, height: size * 0.09).offset(x: size * 0.16, y: size * 0.01)

            case .navOverview:
                roundedRect(x: size * 0.18, y: size * 0.18, w: size * 0.30, h: size * 0.30, r: size * 0.06, color: primary)
                roundedRect(x: size * 0.54, y: size * 0.20, w: size * 0.28, h: size * 0.24, r: size * 0.06, color: primary)
                roundedRect(x: size * 0.18, y: size * 0.54, w: size * 0.28, h: size * 0.24, r: size * 0.06, color: primary)
                roundedRect(x: size * 0.52, y: size * 0.52, w: size * 0.30, h: size * 0.30, r: size * 0.06, color: primary)

            case .navVault:
                roundedRect(x: size * 0.18, y: size * 0.36, w: size * 0.64, h: size * 0.46, r: size * 0.09, color: primary)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.32, y: size * 0.36))
                    p.addQuadCurve(to: CGPoint(x: size * 0.68, y: size * 0.36), control: CGPoint(x: size * 0.50, y: size * 0.12))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
                Circle()
                    .stroke(primary, lineWidth: stroke)
                    .frame(width: size * 0.14, height: size * 0.14)
                    .offset(y: size * 0.12)
                Capsule()
                    .fill(primary)
                    .frame(width: stroke, height: size * 0.12)
                    .offset(y: size * 0.20)

            case .navActivity:
                activityLine(y: 0.30)
                activityLine(y: 0.50)
                activityLine(y: 0.70)

            case .navBudget:
                Circle()
                    .trim(from: 0.10, to: 0.86)
                    .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
                    .rotationEffect(.degrees(-90))
                    .frame(width: size * 0.74, height: size * 0.74)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.50, y: size * 0.40))
                    p.addLine(to: CGPoint(x: size * 0.50, y: size * 0.64))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))

            case .navProviders:
                roundedRect(x: size * 0.24, y: size * 0.24, w: size * 0.50, h: size * 0.50, r: size * 0.08, color: primary)
                roundedRect(x: size * 0.34, y: size * 0.16, w: size * 0.50, h: size * 0.50, r: size * 0.08, color: primary.opacity(0.8))
                roundedRect(x: size * 0.16, y: size * 0.34, w: size * 0.50, h: size * 0.50, r: size * 0.08, color: primary.opacity(0.8))

            case .navSettings:
                Circle().stroke(primary, lineWidth: stroke).frame(width: size * 0.28, height: size * 0.28)
                ForEach(0..<6, id: \.self) { i in
                    Capsule()
                        .fill(primary)
                        .frame(width: stroke, height: size * 0.15)
                        .offset(y: -size * 0.32)
                        .rotationEffect(.degrees(Double(i) * 60.0))
                }
                Circle().stroke(primary, lineWidth: stroke).frame(width: size * 0.70, height: size * 0.70)

            case .setupDocker:
                Path { p in
                    p.move(to: CGPoint(x: size * 0.18, y: size * 0.45))
                    p.addLine(to: CGPoint(x: size * 0.28, y: size * 0.30))
                    p.addLine(to: CGPoint(x: size * 0.78, y: size * 0.30))
                    p.addLine(to: CGPoint(x: size * 0.88, y: size * 0.45))
                    p.addLine(to: CGPoint(x: size * 0.88, y: size * 0.76))
                    p.addLine(to: CGPoint(x: size * 0.18, y: size * 0.76))
                    p.closeSubpath()
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineJoin: .round))
                Path { p in
                    p.move(to: CGPoint(x: size * 0.34, y: size * 0.30))
                    p.addQuadCurve(to: CGPoint(x: size * 0.54, y: size * 0.30), control: CGPoint(x: size * 0.44, y: size * 0.20))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))

            case .setupDaemon:
                Circle().stroke(primary, lineWidth: stroke).frame(width: size * 0.76, height: size * 0.76)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.56, y: size * 0.18))
                    p.addLine(to: CGPoint(x: size * 0.42, y: size * 0.52))
                    p.addLine(to: CGPoint(x: size * 0.56, y: size * 0.52))
                    p.addLine(to: CGPoint(x: size * 0.44, y: size * 0.82))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round, lineJoin: .round))

            case .setupVault:
                Path { p in
                    p.move(to: CGPoint(x: size * 0.28, y: size * 0.68))
                    p.addLine(to: CGPoint(x: size * 0.54, y: size * 0.42))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
                Circle().stroke(primary, lineWidth: stroke).frame(width: size * 0.24, height: size * 0.24).offset(x: -size * 0.20, y: size * 0.20)
                RoundedRectangle(cornerRadius: stroke, style: .continuous)
                    .stroke(primary, lineWidth: stroke)
                    .frame(width: size * 0.22, height: size * 0.10)
                    .rotationEffect(.degrees(-45))
                    .offset(x: size * 0.05, y: size * 0.00)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.70, y: size * 0.30))
                    p.addLine(to: CGPoint(x: size * 0.84, y: size * 0.30))
                    p.move(to: CGPoint(x: size * 0.77, y: size * 0.23))
                    p.addLine(to: CGPoint(x: size * 0.77, y: size * 0.37))
                }
                .stroke(accent, style: StrokeStyle(lineWidth: stroke, lineCap: .round))

            case .setupContainerLaunch:
                Path { p in
                    p.move(to: CGPoint(x: size * 0.32, y: size * 0.80))
                    p.addLine(to: CGPoint(x: size * 0.32, y: size * 0.30))
                    p.addLine(to: CGPoint(x: size * 0.50, y: size * 0.16))
                    p.addLine(to: CGPoint(x: size * 0.68, y: size * 0.30))
                    p.addLine(to: CGPoint(x: size * 0.68, y: size * 0.80))
                    p.closeSubpath()
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round, lineJoin: .round))
                Path { p in
                    p.move(to: CGPoint(x: size * 0.40, y: size * 0.76))
                    p.addLine(to: CGPoint(x: size * 0.34, y: size * 0.90))
                    p.move(to: CGPoint(x: size * 0.60, y: size * 0.76))
                    p.addLine(to: CGPoint(x: size * 0.66, y: size * 0.90))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
                Circle().stroke(primary, lineWidth: stroke).frame(width: size * 0.12, height: size * 0.12).offset(y: size * 0.10)

            case .stepComplete:
                Circle().fill(accent).frame(width: size * 0.84, height: size * 0.84)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.30, y: size * 0.52))
                    p.addLine(to: CGPoint(x: size * 0.44, y: size * 0.66))
                    p.addLine(to: CGPoint(x: size * 0.70, y: size * 0.36))
                }
                .stroke(Color.white, style: StrokeStyle(lineWidth: stroke, lineCap: .round, lineJoin: .round))

            case .stepPending:
                Circle()
                    .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round, dash: [size * 0.16, size * 0.12]))
                    .frame(width: size * 0.84, height: size * 0.84)

            case .stepActive:
                Circle().stroke(primary, lineWidth: stroke).frame(width: size * 0.84, height: size * 0.84)
                Circle().fill(accent).frame(width: size * 0.20, height: size * 0.20)

            case .actionStartSecurely:
                Path { p in
                    p.move(to: CGPoint(x: size * 0.20, y: size * 0.18))
                    p.addLine(to: CGPoint(x: size * 0.20, y: size * 0.82))
                    p.addLine(to: CGPoint(x: size * 0.64, y: size * 0.50))
                    p.closeSubpath()
                }
                .stroke(accent, style: StrokeStyle(lineWidth: stroke, lineJoin: .round))
                shieldOutline(color: accent, inset: size * 0.58, scale: 0.36)

            case .actionStop:
                roundedRect(x: size * 0.25, y: size * 0.25, w: size * 0.50, h: size * 0.50, r: size * 0.10, color: primary)

            case .actionRestart:
                Circle()
                    .trim(from: 0.12, to: 0.92)
                    .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
                    .rotationEffect(.degrees(-70))
                    .frame(width: size * 0.74, height: size * 0.74)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.68, y: size * 0.17))
                    p.addLine(to: CGPoint(x: size * 0.82, y: size * 0.22))
                    p.addLine(to: CGPoint(x: size * 0.72, y: size * 0.32))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round, lineJoin: .round))

            case .actionOpenExternal:
                roundedRect(x: size * 0.18, y: size * 0.30, w: size * 0.54, h: size * 0.54, r: size * 0.08, color: primary)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.46, y: size * 0.56))
                    p.addLine(to: CGPoint(x: size * 0.82, y: size * 0.20))
                    p.move(to: CGPoint(x: size * 0.62, y: size * 0.20))
                    p.addLine(to: CGPoint(x: size * 0.82, y: size * 0.20))
                    p.addLine(to: CGPoint(x: size * 0.82, y: size * 0.40))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round, lineJoin: .round))

            case .actionRetry:
                TokfenceIcon(kind: .actionRestart, size: size, primary: primary, accent: accent, secondary: secondary)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.50, y: size * 0.34))
                    p.addLine(to: CGPoint(x: size * 0.50, y: size * 0.56))
                }
                .stroke(accent, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
                Circle().fill(accent).frame(width: stroke, height: stroke).offset(y: size * 0.20)

            case .menuActive:
                shieldOutline(color: primary)
                Capsule()
                    .fill(primary)
                    .frame(width: stroke, height: size * 0.30)

            case .menuInactive:
                shieldOutline(color: primary)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.18, y: size * 0.82))
                    p.addLine(to: CGPoint(x: size * 0.82, y: size * 0.18))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))

            case .menuAlert:
                shieldOutline(color: primary)
                Capsule()
                    .fill(primary)
                    .frame(width: stroke, height: size * 0.30)
                Circle().fill(primary).frame(width: size * 0.14, height: size * 0.14).offset(x: size * 0.30, y: -size * 0.30)

            case .agentStopped:
                roundedRect(x: size * 0.16, y: size * 0.14, w: size * 0.68, h: size * 0.72, r: size * 0.10, color: primary)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.40, y: size * 0.34))
                    p.addLine(to: CGPoint(x: size * 0.40, y: size * 0.66))
                    p.addLine(to: CGPoint(x: size * 0.64, y: size * 0.50))
                    p.closeSubpath()
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineJoin: .round))

            case .agentRunning:
                roundedRect(x: size * 0.16, y: size * 0.14, w: size * 0.68, h: size * 0.72, r: size * 0.10, color: primary)
                RoundedRectangle(cornerRadius: stroke, style: .continuous)
                    .fill(accent)
                    .frame(width: size * 0.10, height: size * 0.70)
                    .offset(x: -size * 0.29)
                Capsule().fill(primary).frame(width: stroke, height: size * 0.30).offset(x: -size * 0.07)
                Capsule().fill(primary).frame(width: stroke, height: size * 0.30).offset(x: size * 0.07)

            case .agentError:
                roundedRect(x: size * 0.16, y: size * 0.14, w: size * 0.68, h: size * 0.72, r: size * 0.10, color: primary)
                RoundedRectangle(cornerRadius: stroke, style: .continuous)
                    .fill(accent)
                    .frame(width: size * 0.10, height: size * 0.70)
                    .offset(x: -size * 0.29)
                Capsule().fill(primary).frame(width: stroke, height: size * 0.26).offset(y: -size * 0.06)
                Circle().fill(primary).frame(width: stroke, height: stroke).offset(y: size * 0.14)

            case .agentStarting:
                Circle()
                    .trim(from: 0.08, to: 0.83)
                    .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
                    .rotationEffect(.degrees(-90))
                    .frame(width: size * 0.78, height: size * 0.78)

            case .providerAnthropic:
                Text("A")
                    .font(.system(size: size * 0.74, weight: .semibold, design: .rounded))
                    .foregroundStyle(primary)

            case .providerGoogle:
                Text("G")
                    .font(.system(size: size * 0.74, weight: .semibold, design: .rounded))
                    .foregroundStyle(primary)

            case .providerMistral:
                Text("M")
                    .font(.system(size: size * 0.74, weight: .semibold, design: .rounded))
                    .foregroundStyle(primary)

            case .providerOpenAI:
                Path { p in
                    p.move(to: CGPoint(x: size * 0.50, y: size * 0.12))
                    p.addLine(to: CGPoint(x: size * 0.74, y: size * 0.24))
                    p.addLine(to: CGPoint(x: size * 0.74, y: size * 0.50))
                    p.addLine(to: CGPoint(x: size * 0.50, y: size * 0.62))
                    p.addLine(to: CGPoint(x: size * 0.26, y: size * 0.50))
                    p.addLine(to: CGPoint(x: size * 0.26, y: size * 0.24))
                    p.closeSubpath()
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineJoin: .round))
                Path { p in
                    p.move(to: CGPoint(x: size * 0.50, y: size * 0.30))
                    p.addLine(to: CGPoint(x: size * 0.62, y: size * 0.36))
                    p.addLine(to: CGPoint(x: size * 0.62, y: size * 0.50))
                    p.addLine(to: CGPoint(x: size * 0.50, y: size * 0.56))
                    p.addLine(to: CGPoint(x: size * 0.38, y: size * 0.50))
                    p.addLine(to: CGPoint(x: size * 0.38, y: size * 0.36))
                    p.closeSubpath()
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineJoin: .round))

            case .providerGeneric:
                Circle().stroke(primary, lineWidth: stroke).frame(width: size * 0.42, height: size * 0.42)
                Path { p in
                    p.move(to: CGPoint(x: size * 0.44, y: size * 0.66))
                    p.addLine(to: CGPoint(x: size * 0.44, y: size * 0.86))
                    p.move(to: CGPoint(x: size * 0.56, y: size * 0.66))
                    p.addLine(to: CGPoint(x: size * 0.56, y: size * 0.86))
                }
                .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
            }
        }
        .frame(width: size, height: size)
        .accessibilityHidden(true)
    }

    private func shieldOutline(color: Color, inset: CGFloat = 0, scale: CGFloat = 1) -> some View {
        let s = size * scale
        let ox = inset
        return Path { p in
            p.move(to: CGPoint(x: ox + s * 0.50, y: ox + s * 0.10))
            p.addLine(to: CGPoint(x: ox + s * 0.84, y: ox + s * 0.24))
            p.addLine(to: CGPoint(x: ox + s * 0.84, y: ox + s * 0.56))
            p.addQuadCurve(to: CGPoint(x: ox + s * 0.50, y: ox + s * 0.90), control: CGPoint(x: ox + s * 0.74, y: ox + s * 0.84))
            p.addQuadCurve(to: CGPoint(x: ox + s * 0.16, y: ox + s * 0.56), control: CGPoint(x: ox + s * 0.26, y: ox + s * 0.84))
            p.addLine(to: CGPoint(x: ox + s * 0.16, y: ox + s * 0.24))
            p.closeSubpath()
        }
        .stroke(color, style: StrokeStyle(lineWidth: stroke, lineCap: .round, lineJoin: .round))
    }

    private func roundedRect(x: CGFloat, y: CGFloat, w: CGFloat, h: CGFloat, r: CGFloat, color: Color) -> some View {
        RoundedRectangle(cornerRadius: r, style: .continuous)
            .stroke(color, lineWidth: stroke)
            .frame(width: w, height: h)
            .position(x: x + w / 2, y: y + h / 2)
    }

    private func activityLine(y: CGFloat) -> some View {
        ZStack {
            Path { p in
                p.move(to: CGPoint(x: size * 0.18, y: size * y))
                p.addLine(to: CGPoint(x: size * 0.70, y: size * y))
            }
            .stroke(primary, style: StrokeStyle(lineWidth: stroke, lineCap: .round))
            Circle().fill(primary).frame(width: size * 0.10, height: size * 0.10).offset(x: size * 0.29, y: size * (y - 0.5))
        }
    }
}

struct TokfenceSymbol: View {
    let systemName: String
    var size: CGFloat
    var color: Color
    var accent: Color? = nil

    var body: some View {
        if let mapped = TokfenceSymbol.map(systemName) {
            TokfenceIcon(
                kind: mapped,
                size: size,
                primary: color,
                accent: accent ?? color,
                secondary: color.opacity(0.85)
            )
        } else {
            Image(systemName: systemName)
                .font(.system(size: size, weight: .medium))
                .foregroundStyle(color)
        }
    }

    static func map(_ name: String) -> TokfenceIconKind? {
        switch name {
        case "cpu.fill":
            return .navAgents
        case "gauge.with.dots.needle.33percent":
            return .navOverview
        case "key.fill":
            return .navVault
        case "list.bullet.rectangle":
            return .navActivity
        case "dollarsign.circle":
            return .navBudget
        case "server.rack":
            return .navProviders
        case "gearshape":
            return .navSettings
        case "play.fill":
            return .actionStartSecurely
        case "stop.fill":
            return .actionStop
        case "arrow.clockwise":
            return .actionRestart
        case "arrow.up.right.square", "rectangle.and.cursor.arrow":
            return .actionOpenExternal
        default:
            return nil
        }
    }

    static func providerKind(_ provider: String) -> TokfenceIconKind {
        switch provider.lowercased() {
        case "anthropic":
            return .providerAnthropic
        case "openai":
            return .providerOpenAI
        case "google":
            return .providerGoogle
        case "mistral":
            return .providerMistral
        default:
            return .providerGeneric
        }
    }
}
