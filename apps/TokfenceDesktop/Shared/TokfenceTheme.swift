import SwiftUI

enum TokfenceTheme {
    static let bgPrimary = Color(light: 0xFFFFFF, dark: 0x1C1C1E)
    static let bgSecondary = Color(light: 0xF5F5F7, dark: 0x2C2C2E)
    static let bgTertiary = Color(light: 0xE8E8ED, dark: 0x3A3A3C)
    static let bgElevated = Color(light: 0xFFFFFF, dark: 0x2C2C2E)

    static let accentPrimary = Color(hex: 0xD4501E)
    static let accentHover = Color(hex: 0xB8431A)
    static let accentMuted = Color(hex: 0xD4501E).opacity(0.12)

    static let healthy = Color(nsColor: .systemGreen)
    static let warning = Color(nsColor: .systemOrange)
    static let danger = Color(nsColor: .systemRed)
    static let info = Color(nsColor: .systemBlue)

    static let textPrimary = Color(light: 0x1D1D1F, dark: 0xF5F5F7)
    static let textSecondary = Color(light: 0x86868B, dark: 0x98989D)
    static let textTertiary = Color(light: 0xAEAEB2, dark: 0x636366)

    static let cardCorner: CGFloat = 8
    static let buttonCorner: CGFloat = 6
    static let badgeCorner: CGFloat = 4

    static let spaceXs: CGFloat = 4
    static let spaceSm: CGFloat = 8
    static let spaceMd: CGFloat = 16
    static let spaceLg: CGFloat = 24
    static let spaceXl: CGFloat = 32

    static let sectionTransition = AnyTransition.opacity.combined(with: .move(edge: .trailing))
    static let navTransition = AnyTransition.move(edge: .leading).combined(with: .opacity)
    static let uiAnimation = Animation.easeInOut(duration: 0.2)

    static func statusColor(for statusCode: Int) -> Color {
        if statusCode >= 200 && statusCode < 300 {
            return healthy
        }
        if statusCode == 429 {
            return warning
        }
        if statusCode >= 400 {
            return danger
        }
        return textTertiary
    }

    static func budgetColor(progress: Double) -> Color {
        if progress >= 1.0 {
            return danger
        }
        if progress >= 0.8 {
            return warning
        }
        return healthy
    }

    static func providerColor(_ provider: String) -> Color {
        switch provider.lowercased() {
        case "anthropic":
            return Color(hex: 0xA14500)
        case "openai":
            return Color(hex: 0x107A65)
        case "google":
            return Color(hex: 0x1A73E8)
        case "mistral":
            return Color(hex: 0xE08100)
        case "groq":
            return Color(hex: 0x34343A)
        case "openrouter":
            return Color(hex: 0x4C58D0)
        default:
            return Color(hex: 0x8A8A90)
        }
    }
}

private extension Color {
    init(hex: Int) {
        self.init(
            red: Double((hex >> 16) & 0xFF) / 255.0,
            green: Double((hex >> 8) & 0xFF) / 255.0,
            blue: Double(hex & 0xFF) / 255.0
        )
    }

    init(light: Int, dark: Int) {
        self.init(NSColor(name: nil) { appearance in
            switch appearance.bestMatch(from: [.darkAqua, .aqua]) {
            case .darkAqua:
                return NSColor(
                    red: CGFloat((dark >> 16) & 0xFF) / 255.0,
                    green: CGFloat((dark >> 8) & 0xFF) / 255.0,
                    blue: CGFloat(dark & 0xFF) / 255.0,
                    alpha: 1
                )
            default:
                return NSColor(
                    red: CGFloat((light >> 16) & 0xFF) / 255.0,
                    green: CGFloat((light >> 8) & 0xFF) / 255.0,
                    blue: CGFloat(light & 0xFF) / 255.0,
                    alpha: 1
                )
            }
        })
    }
}
