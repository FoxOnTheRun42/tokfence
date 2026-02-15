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
        let snapshot = TokfenceSharedStore.loadSnapshot()
        completion(TokfenceWidgetEntry(date: Date(), snapshot: snapshot))
    }

    func getTimeline(in context: Context, completion: @escaping (Timeline<TokfenceWidgetEntry>) -> Void) {
        let snapshot = TokfenceSharedStore.loadSnapshot()
        let entry = TokfenceWidgetEntry(date: Date(), snapshot: snapshot)
        let nextRefresh = Date().addingTimeInterval(60)
        completion(Timeline(entries: [entry], policy: .after(nextRefresh)))
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

    private var small: some View {
        VStack(alignment: .leading, spacing: 6) {
            Text(entry.snapshot.running ? "Tokfence Online" : "Tokfence Offline")
                .font(.system(size: 12, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
            Text(TokfenceFormatting.usd(cents: entry.snapshot.todayCostCents))
                .font(.system(size: 20, weight: .heavy, design: .rounded))
                .foregroundStyle(.white)
            Text("\(entry.snapshot.todayRequests) requests today")
                .font(.system(size: 11, weight: .semibold, design: .rounded))
                .foregroundStyle(.white.opacity(0.8))
            Spacer()
            HStack {
                Image(systemName: "shield.lefthalf.filled")
                Text(entry.snapshot.revokedProviders.isEmpty ? "All providers active" : "\(entry.snapshot.revokedProviders.count) revoked")
            }
            .font(.system(size: 10, weight: .medium, design: .rounded))
            .foregroundStyle(.white.opacity(0.8))
        }
        .padding(12)
        .containerBackground(for: .widget) {
            LinearGradient(colors: [Color(red: 0.09, green: 0.19, blue: 0.34), Color(red: 0.15, green: 0.12, blue: 0.30)], startPoint: .topLeading, endPoint: .bottomTrailing)
        }
    }

    private var medium: some View {
        HStack(spacing: 12) {
            VStack(alignment: .leading, spacing: 6) {
                Text(entry.snapshot.running ? "Tokfence Running" : "Tokfence Offline")
                    .font(.system(size: 13, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                Text("Cost Today")
                    .font(.system(size: 11, weight: .medium, design: .rounded))
                    .foregroundStyle(.white.opacity(0.8))
                Text(TokfenceFormatting.usd(cents: entry.snapshot.todayCostCents))
                    .font(.system(size: 26, weight: .heavy, design: .rounded))
                    .foregroundStyle(.white)
                Text("\(entry.snapshot.todayRequests) requests · In \(TokfenceFormatting.tokens(entry.snapshot.todayInputTokens)) · Out \(TokfenceFormatting.tokens(entry.snapshot.todayOutputTokens))")
                    .font(.system(size: 10, weight: .medium, design: .rounded))
                    .foregroundStyle(.white.opacity(0.8))
            }
            Spacer()
            VStack(alignment: .trailing, spacing: 6) {
                Text("Budgets")
                    .font(.system(size: 12, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                if let budget = entry.snapshot.budgets.first {
                    let progress = TokfenceFormatting.percent(current: budget.currentSpendCents, limit: budget.limitCents)
                    Text(budget.provider)
                        .font(.system(size: 11, weight: .semibold, design: .rounded))
                        .foregroundStyle(.white.opacity(0.9))
                    ProgressView(value: progress)
                        .tint(progress < 0.8 ? .green : (progress < 1.0 ? .orange : .red))
                        .frame(width: 100)
                    Text("\(Int(progress * 100))%")
                        .font(.system(size: 10, weight: .semibold, design: .rounded))
                        .foregroundStyle(.white.opacity(0.8))
                } else {
                    Text("No budgets")
                        .font(.system(size: 11, weight: .semibold, design: .rounded))
                        .foregroundStyle(.white.opacity(0.85))
                }
            }
        }
        .padding(14)
        .containerBackground(for: .widget) {
            LinearGradient(colors: [Color(red: 0.09, green: 0.19, blue: 0.34), Color(red: 0.15, green: 0.12, blue: 0.30)], startPoint: .topLeading, endPoint: .bottomTrailing)
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
        .description("See Tokfence status, spend, and control posture at a glance.")
        .supportedFamilies([.systemSmall, .systemMedium])
    }
}

@main
struct TokfenceWidgetBundle: WidgetBundle {
    var body: some Widget {
        TokfenceWidget()
    }
}
