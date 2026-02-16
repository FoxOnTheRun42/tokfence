import SwiftUI

struct TokfenceCard<Content: View>: View {
    let content: Content

    init(@ViewBuilder content: () -> Content) {
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            content
        }
        .padding(16)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(TokfenceTheme.bgSecondary, in: RoundedRectangle(cornerRadius: TokfenceTheme.cardCorner, style: .continuous))
    }
}

struct TokfenceSurface<Content: View>: View {
    let content: Content
    let padding: CGFloat

    init(padding: CGFloat = 16, @ViewBuilder content: () -> Content) {
        self.padding = padding
        self.content = content()
    }

    var body: some View {
        VStack(alignment: .leading, spacing: TokfenceTheme.spaceMd) {
            content
        }
        .padding(padding)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(TokfenceTheme.bgSecondary, in: RoundedRectangle(cornerRadius: TokfenceTheme.cardCorner, style: .continuous))
    }
}

struct TokfenceSectionHeader: View {
    let title: String
    var subtitle: String?
    var trailing: AnyView?

    var body: some View {
        HStack(alignment: .firstTextBaseline, spacing: TokfenceTheme.spaceSm) {
            VStack(alignment: .leading, spacing: 2) {
                Text(title)
                    .font(.system(size: 20, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.textPrimary)
                if let subtitle, !subtitle.isEmpty {
                    Text(subtitle)
                        .font(.system(size: 11, weight: .regular))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }
            }
            Spacer()
            trailing
        }
    }
}

struct TokfenceSectionRow: View {
    let title: String
    let value: String

    var body: some View {
        HStack {
            Text(title)
                .font(.system(size: TokfenceTheme.fontBody, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
            Spacer()
            Text(value)
                .font(.system(size: TokfenceTheme.fontBody, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
        }
    }
}

struct TokfenceStatusDot: View {
    let color: Color
    let label: String

    var body: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(color)
                .frame(width: 8, height: 8)
            Text(label)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
        }
    }
}

struct TokfenceLiveBadge: View {
    let text: String
    let color: Color
    let isActive: Bool

    var body: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(color)
                .frame(width: 7, height: 7)
                .scaleEffect(isActive ? 1.15 : 1)
                .animation(
                    isActive ? TokfenceTheme.uiAnimation.repeatForever(autoreverses: true) : nil,
                    value: isActive
                )
            Text(text)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
        }
        .padding(.vertical, 4)
        .padding(.horizontal, 8)
        .background(
            RoundedRectangle(cornerRadius: TokfenceTheme.badgeCorner, style: .continuous)
                .fill(TokfenceTheme.textTertiary.opacity(0.12))
        )
    }
}

struct TokfenceToast: View {
    let message: String
    let tone: Color
    let action: String?
    let onClose: (() -> Void)?

    var body: some View {
        HStack(spacing: TokfenceTheme.spaceSm) {
            Circle()
                .fill(tone)
                .frame(width: 8, height: 8)
            Text(message)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(.white)
            Spacer(minLength: 0)
            if let action, !action.isEmpty {
                Button(action) {
                    onClose?()
                }
                .font(.system(size: 11, weight: .semibold))
                .buttonStyle(.plain)
                .foregroundStyle(.white.opacity(0.9))
            }
            Button {
                onClose?()
            } label: {
                Image(systemName: "xmark")
            }
            .buttonStyle(.plain)
            .foregroundStyle(.white.opacity(0.9))
        }
        .padding(.horizontal, TokfenceTheme.spaceMd)
        .padding(.vertical, 8)
        .background(tone.opacity(0.95), in: RoundedRectangle(cornerRadius: 8, style: .continuous))
        .contentShape(RoundedRectangle(cornerRadius: 8, style: .continuous))
        .shadow(color: .black.opacity(0.15), radius: 10, x: 0, y: 4)
    }
}

struct TokfenceProviderBadge: View {
    let provider: String
    let active: Bool

    var body: some View {
        Text(TokfenceFormatting.providerLabel(provider))
            .font(.system(size: 11, weight: .semibold))
            .foregroundStyle(active ? Color.white : TokfenceTheme.textSecondary)
            .padding(.vertical, 3)
            .padding(.horizontal, 8)
            .background(
                RoundedRectangle(cornerRadius: TokfenceTheme.badgeCorner, style: .continuous)
                    .fill(active ? TokfenceTheme.providerColor(provider) : TokfenceTheme.bgTertiary)
            )
    }
}

struct TokfenceBudgetProgressBar: View {
    let current: Int64
    let limit: Int64

    var body: some View {
        let progress = TokfenceFormatting.percent(current: current, limit: limit)
        let capped = min(progress, 1.0)
        let color = TokfenceTheme.budgetColor(progress: progress)
        VStack(alignment: .leading, spacing: 4) {
            GeometryReader { proxy in
                ZStack(alignment: .leading) {
                    RoundedRectangle(cornerRadius: 4, style: .continuous)
                        .fill(TokfenceTheme.bgTertiary)
                    RoundedRectangle(cornerRadius: 4, style: .continuous)
                        .fill(color)
                        .frame(width: proxy.size.width * capped)
                }
            }
            .frame(height: 8)
            Text(TokfenceFormatting.percentString(current: current, limit: limit))
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
        }
    }
}

struct TokfenceEmptyState: View {
    let symbol: String
    let title: String
    let message: String
    let actionTitle: String?
    let action: (() -> Void)?

    var body: some View {
        VStack(spacing: 10) {
            Image(systemName: symbol)
                .font(.system(size: 24, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
            Text(title)
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text(message)
                .font(.system(size: 12, weight: .regular))
                .foregroundStyle(TokfenceTheme.textSecondary)
                .multilineTextAlignment(.center)
            if let actionTitle, let action {
                Button(actionTitle, action: action)
                    .buttonStyle(.borderedProminent)
                    .tint(TokfenceTheme.accentPrimary)
            }
        }
        .frame(maxWidth: .infinity)
        .padding(24)
        .background(TokfenceTheme.bgSecondary, in: RoundedRectangle(cornerRadius: TokfenceTheme.cardCorner, style: .continuous))
    }
}

struct TokfenceNavItem: View {
    let isSelected: Bool
    let title: String
    let icon: String
    let action: () -> Void

    var body: some View {
        Button(action: action) {
            HStack(spacing: TokfenceTheme.spaceSm) {
                Image(systemName: icon)
                    .font(.system(size: 14, weight: .medium))
                    .frame(width: 16)
                    .foregroundStyle(isSelected ? TokfenceTheme.accentPrimary : TokfenceTheme.textTertiary)
                Text(title)
                    .font(.system(size: TokfenceTheme.fontBody, weight: isSelected ? .semibold : .medium))
                    .foregroundStyle(isSelected ? TokfenceTheme.textPrimary : TokfenceTheme.textSecondary)
                    .frame(maxWidth: .infinity, alignment: .leading)
            }
            .padding(.horizontal, 12)
            .padding(.vertical, 8)
            .background(
                RoundedRectangle(cornerRadius: TokfenceTheme.badgeCorner, style: .continuous)
                    .fill(isSelected ? TokfenceTheme.accentMuted : TokfenceTheme.bgTertiary)
            )
        }
        .buttonStyle(.plain)
        .contentShape(Rectangle())
    }
}

struct TokfenceMetricCard: View {
    let icon: String
    let value: String
    let title: String
    let subtitle: String
    var trend: String = ""

    var body: some View {
        TokfenceCard {
            HStack(spacing: 8) {
                Image(systemName: icon)
                    .font(.system(size: 14, weight: .medium))
                    .foregroundStyle(TokfenceTheme.accentPrimary)
                Spacer()
            }
            Text(value)
                .font(.system(size: 24, weight: .semibold, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text(title)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
            Text(subtitle)
                .font(.system(size: 10, weight: .regular))
                .foregroundStyle(TokfenceTheme.textTertiary)
            if !trend.isEmpty {
                Text(trend)
                    .font(.system(size: 10, weight: .regular))
                    .foregroundStyle(TokfenceTheme.textSecondary)
            }
        }
        .frame(height: 140)
    }
}

struct TokfenceStatusBadge: View {
    let label: String
    let icon: String
    let tint: Color

    var body: some View {
        HStack(spacing: 6) {
            Circle()
                .fill(tint)
                .frame(width: 8, height: 8)
            Text(label)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(
            RoundedRectangle(cornerRadius: TokfenceTheme.badgeCorner, style: .continuous)
                .fill(TokfenceTheme.textTertiary.opacity(0.18))
        )
    }
}
