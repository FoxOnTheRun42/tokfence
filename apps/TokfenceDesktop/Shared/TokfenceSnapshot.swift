import Foundation

struct TokfenceBudget: Codable, Hashable, Identifiable {
    let provider: String
    let limitCents: Int64
    let period: String
    let currentSpendCents: Int64
    let periodStart: Date
    let enabled: Bool

    var id: String { provider }

    enum CodingKeys: String, CodingKey {
        case provider
        case limitCents = "limit_cents"
        case period
        case currentSpendCents = "current_spend_cents"
        case periodStart = "period_start"
        case enabled
    }
}

struct TokfenceSnapshot: Codable, Hashable {
    let generatedAt: Date
    let running: Bool
    let pid: Int?
    let addr: String?
    let todayRequests: Int
    let todayInputTokens: Int64
    let todayOutputTokens: Int64
    let todayCostCents: Int64
    let topProvider: String?
    let topProviderCents: Int64?
    let budgets: [TokfenceBudget]
    let revokedProviders: [String]
    let vaultProviders: [String]
    let lastRequestAt: String?
    let warnings: [String]

    enum CodingKeys: String, CodingKey {
        case generatedAt = "generated_at"
        case running
        case pid
        case addr
        case todayRequests = "today_requests"
        case todayInputTokens = "today_input_tokens"
        case todayOutputTokens = "today_output_tokens"
        case todayCostCents = "today_cost_cents"
        case topProvider = "top_provider"
        case topProviderCents = "top_provider_cost_cents"
        case budgets
        case revokedProviders = "revoked_providers"
        case vaultProviders = "vault_providers"
        case lastRequestAt = "last_request_at"
        case warnings
    }

    static var empty: TokfenceSnapshot {
        TokfenceSnapshot(
            generatedAt: Date(),
            running: false,
            pid: nil,
            addr: nil,
            todayRequests: 0,
            todayInputTokens: 0,
            todayOutputTokens: 0,
            todayCostCents: 0,
            topProvider: nil,
            topProviderCents: nil,
            budgets: [],
            revokedProviders: [],
            vaultProviders: [],
            lastRequestAt: nil,
            warnings: []
        )
    }
}

extension JSONDecoder {
    static let tokfence: JSONDecoder = {
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return decoder
    }()
}

extension JSONEncoder {
    static let tokfence: JSONEncoder = {
        let encoder = JSONEncoder()
        encoder.dateEncodingStrategy = .iso8601
        return encoder
    }()
}
