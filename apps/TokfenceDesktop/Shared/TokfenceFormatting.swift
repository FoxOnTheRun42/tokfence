import Foundation

enum TokfenceFormatting {
    static func usd(cents: Int64) -> String {
        String(format: "$%.2f", Double(cents) / 100.0)
    }

    static func usdDouble(cents: Int64) -> Double {
        Double(cents) / 100.0
    }

    static func tokens(_ value: Int64) -> String {
        switch value {
        case 1_000_000...:
            return String(format: "%.2fM", Double(value) / 1_000_000.0)
        case 1_000...:
            return String(format: "%.1fk", Double(value) / 1_000.0)
        default:
            return "\(value)"
        }
    }

    static func percent(current: Int64, limit: Int64) -> Double {
        guard limit > 0 else { return 0 }
        return max(0, Double(current) / Double(limit))
    }

    static func percentString(current: Int64, limit: Int64) -> String {
        guard limit > 0 else { return "0%" }
        return String(format: "%.1f%%", percent(current: current, limit: limit) * 100.0)
    }

    static func latency(ms: Int) -> String {
        if ms >= 1_000 {
            return String(format: "%.2fs", Double(ms) / 1_000.0)
        }
        return "\(ms)ms"
    }

    static func timeOfDay(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateFormat = "HH:mm:ss"
        return formatter.string(from: date)
    }

    static func shortDateTime(_ date: Date) -> String {
        let formatter = DateFormatter()
        formatter.dateFormat = "MMM d, HH:mm"
        return formatter.string(from: date)
    }

    static func relative(_ date: Date) -> String {
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .short
        return formatter.localizedString(for: date, relativeTo: Date())
    }

    static func budgetResetText(from start: Date, period: String) -> String {
        let resetAt: Date
        switch period.lowercased() {
        case "monthly":
            resetAt = Calendar.current.date(byAdding: .month, value: 1, to: start) ?? start
        default:
            resetAt = Calendar.current.date(byAdding: .day, value: 1, to: start) ?? start
        }
        return relative(resetAt)
    }

    static func providerLabel(_ provider: String) -> String {
        switch provider.lowercased() {
        case "openai":
            return "OpenAI"
        case "openrouter":
            return "OpenRouter"
        default:
            return provider.capitalized
        }
    }

    static func projectedSpend(currentCents: Int64, periodStart: Date, period: String) -> Int64 {
        let elapsed = max(Date().timeIntervalSince(periodStart), 60)
        let full: TimeInterval
        switch period.lowercased() {
        case "monthly":
            let start = periodStart
            full = (Calendar.current.date(byAdding: .month, value: 1, to: start) ?? start).timeIntervalSince(start)
        default:
            full = 24 * 60 * 60
        }
        if full <= 0 {
            return currentCents
        }
        let rate = Double(currentCents) / elapsed
        return Int64(max(Double(currentCents), rate * full))
    }
}
