import SwiftUI
import WidgetKit

struct TokfenceWidgetEntry: TimelineEntry {
    let date: Date
    let snapshot: TokfenceSnapshot
}

struct TokfenceProvider: TimelineProvider {
    func placeholder(in context: Context) -> TokfenceWidgetEntry {
        TokfenceWidgetEntry(date: Date(), snapshot: .empty)
    }

    func getSnapshot(in context: Context, completion: @escaping (TokfenceWidgetEntry) -> Void) {
        completion(TokfenceWidgetEntry(date: Date(), snapshot: TokfenceSharedStore.loadSnapshot()))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<TokfenceWidgetEntry>) -> Void) {
        let entry = TokfenceWidgetEntry(date: Date(), snapshot: TokfenceSharedStore.loadSnapshot())
        completion(Timeline(entries: [entry], policy: .after(Date().addingTimeInterval(60))))
    }
}

struct TokfenceWidgetEntryView: View {
    @Environment(\.widgetFamily) private var family
    let entry: TokfenceWidgetEntry

    var body: some View {
        switch family {
        case .systemSmall:
            small
        default:
            medium
        }
    }

    private var dailyBudget: TokfenceBudget? {
        entry.snapshot.budgets.first(where: { $0.provider == "global" && $0.period.lowercased() == "daily" })
    }

    private var providersLine: String {
        let active = entry.snapshot.providers.filter { !entry.snapshot.revokedProviders.contains($0) }
        if active.isEmpty {
            return "no providers"
        }
        return active.prefix(3).map(TokfenceFormatting.providerLabel).joined(separator: " · ")
    }

    private var small: some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack {
                Text("Tokfence")
                    .font(.system(size: 12, weight: .semibold))
                Spacer()
                Circle()
                    .fill(entry.snapshot.running ? TokfenceTheme.healthy : TokfenceTheme.danger)
                    .frame(width: 8, height: 8)
            }

            Text(TokfenceFormatting.usd(cents: entry.snapshot.todayCostCents))
                .font(.system(size: 22, weight: .semibold, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text("\(entry.snapshot.todayRequests) requests")
                .font(.system(size: 11, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textSecondary)

            if let budget = dailyBudget {
                WidgetBudgetBar(current: budget.currentSpendCents, limit: budget.limitCents)
            } else {
                Text("No daily budget")
                    .font(.system(size: 10, weight: .regular))
                    .foregroundStyle(TokfenceTheme.textSecondary)
            }
        }
        .padding(12)
        .containerBackground(for: .widget) {
            TokfenceTheme.bgSecondary
        }
    }

    private var medium: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                Text("Tokfence")
                    .font(.system(size: 13, weight: .semibold))
                Circle()
                    .fill(entry.snapshot.running ? TokfenceTheme.healthy : TokfenceTheme.danger)
                    .frame(width: 8, height: 8)
                Text(entry.snapshot.running ? "Online" : "Offline")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(TokfenceTheme.textSecondary)
                Spacer()
                if entry.snapshot.killSwitchActive {
                    Text("KILLED")
                        .font(.system(size: 10, weight: .semibold))
                        .padding(.vertical, 2)
                        .padding(.horizontal, 6)
                        .background(TokfenceTheme.danger, in: RoundedRectangle(cornerRadius: 4, style: .continuous))
                        .foregroundStyle(.white)
                }
            }

            HStack(spacing: 16) {
                metric(title: "cost", value: TokfenceFormatting.usd(cents: entry.snapshot.todayCostCents))
                metric(title: "requests", value: "\(entry.snapshot.todayRequests)")
                metric(title: "tokens", value: TokfenceFormatting.tokens(entry.snapshot.todayInputTokens + entry.snapshot.todayOutputTokens))
            }

            if let budget = dailyBudget {
                HStack {
                    Text("Budget")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                    WidgetBudgetBar(current: budget.currentSpendCents, limit: budget.limitCents)
                }
            }

            Text(providersLine)
                .font(.system(size: 10, weight: .regular))
                .foregroundStyle(TokfenceTheme.textSecondary)
                .lineLimit(1)
        }
        .padding(12)
        .containerBackground(for: .widget) {
            TokfenceTheme.bgSecondary
        }
    }

    private func metric(title: String, value: String) -> some View {
        VStack(alignment: .leading, spacing: 2) {
            Text(value)
                .font(.system(size: 14, weight: .semibold, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text(title)
                .font(.system(size: 10, weight: .regular))
                .foregroundStyle(TokfenceTheme.textSecondary)
        }
    }
}

struct TokfenceWidget: Widget {
    let kind: String = "TokfenceWidget"

    var body: some WidgetConfiguration {
        StaticConfiguration(kind: kind, provider: TokfenceProvider()) { entry in
            TokfenceWidgetEntryView(entry: entry)
        }
        .configurationDisplayName("Tokfence")
        .description("Local cost, usage and kill switch status at a glance.")
        .supportedFamilies([.systemSmall, .systemMedium])
    }
}

private struct WidgetBudgetBar: View {
    let current: Int64
    let limit: Int64

    var body: some View {
        let progress = TokfenceFormatting.percent(current: current, limit: limit)
        let capped = min(progress, 1.0)
        let color = TokfenceTheme.budgetColor(progress: progress)
        GeometryReader { proxy in
            ZStack(alignment: .leading) {
                RoundedRectangle(cornerRadius: 3, style: .continuous)
                    .fill(TokfenceTheme.bgTertiary)
                RoundedRectangle(cornerRadius: 3, style: .continuous)
                    .fill(color)
                    .frame(width: proxy.size.width * capped)
            }
        }
        .frame(height: 7)
    }
}

@main
struct TokfenceWidgetBundle: WidgetBundle {
    var body: some Widget {
        TokfenceWidget()
    }
}
