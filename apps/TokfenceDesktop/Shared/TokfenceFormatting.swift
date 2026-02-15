import Foundation

enum TokfenceFormatting {
    static func usd(cents: Int64) -> String {
        String(format: "$%.2f", Double(cents) / 100.0)
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
        return min(1.0, max(0, Double(current) / Double(limit)))
    }
}
