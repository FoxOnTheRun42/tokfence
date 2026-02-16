import Foundation

struct TokfenceBudget: Codable, Hashable, Identifiable {
    let provider: String
    let limitCents: Int64
    let period: String
    let currentSpendCents: Int64
    let periodStart: Date
    let enabled: Bool

    var id: String { provider }

    private struct SnakeCaseBudget: Codable {
        let provider: String
        let limitCents: Int64
        let period: String
        let currentSpendCents: Int64
        let periodStart: Date
        let enabled: Bool

        enum CodingKeys: String, CodingKey {
            case provider
            case limitCents = "limit_cents"
            case period
            case currentSpendCents = "current_spend_cents"
            case periodStart = "period_start"
            case enabled
        }
    }

    private struct PascalCaseBudget: Codable {
        let provider: String
        let limitCents: Int64
        let period: String
        let currentSpendCents: Int64
        let periodStart: Date
        let enabled: Bool

        enum CodingKeys: String, CodingKey {
            case provider = "Provider"
            case limitCents = "LimitCents"
            case period = "Period"
            case currentSpendCents = "CurrentSpendCents"
            case periodStart = "PeriodStart"
            case enabled = "Enabled"
        }
    }

    init(from decoder: Decoder) throws {
        if let snake = try? SnakeCaseBudget(from: decoder) {
            provider = snake.provider
            limitCents = snake.limitCents
            period = snake.period
            currentSpendCents = snake.currentSpendCents
            periodStart = snake.periodStart
            enabled = snake.enabled
            return
        }
        let pascal = try PascalCaseBudget(from: decoder)
        provider = pascal.provider
        limitCents = pascal.limitCents
        period = pascal.period
        currentSpendCents = pascal.currentSpendCents
        periodStart = pascal.periodStart
        enabled = pascal.enabled
    }

    func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: SnakeCaseBudget.CodingKeys.self)
        try container.encode(provider, forKey: .provider)
        try container.encode(limitCents, forKey: .limitCents)
        try container.encode(period, forKey: .period)
        try container.encode(currentSpendCents, forKey: .currentSpendCents)
        try container.encode(periodStart, forKey: .periodStart)
        try container.encode(enabled, forKey: .enabled)
    }
}

struct TokfenceProviderTodayStats: Codable, Hashable {
    let requestCount: Int
    let inputTokens: Int64
    let outputTokens: Int64
    let estimatedCostCents: Int64

    enum CodingKeys: String, CodingKey {
        case requestCount = "request_count"
        case inputTokens = "input_tokens"
        case outputTokens = "output_tokens"
        case estimatedCostCents = "estimated_cost_cents"
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
    let providers: [String]
    let providerUpstreams: [String: String]
    let rateLimits: [String: Int]
    let killSwitchActive: Bool
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
        case providers
        case providerUpstreams = "provider_upstreams"
        case rateLimits = "rate_limits"
        case killSwitchActive = "kill_switch_active"
        case lastRequestAt = "last_request_at"
        case warnings
    }

    init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        generatedAt = (try? container.decode(Date.self, forKey: .generatedAt)) ?? Date()
        running = (try? container.decode(Bool.self, forKey: .running)) ?? false
        pid = try? container.decode(Int.self, forKey: .pid)
        addr = try? container.decode(String.self, forKey: .addr)
        todayRequests = (try? container.decode(Int.self, forKey: .todayRequests)) ?? 0
        todayInputTokens = (try? container.decode(Int64.self, forKey: .todayInputTokens)) ?? 0
        todayOutputTokens = (try? container.decode(Int64.self, forKey: .todayOutputTokens)) ?? 0
        todayCostCents = (try? container.decode(Int64.self, forKey: .todayCostCents)) ?? 0
        topProvider = try? container.decode(String.self, forKey: .topProvider)
        topProviderCents = try? container.decode(Int64.self, forKey: .topProviderCents)
        budgets = (try? container.decode([TokfenceBudget].self, forKey: .budgets)) ?? []
        revokedProviders = (try? container.decode([String].self, forKey: .revokedProviders)) ?? []
        vaultProviders = (try? container.decode([String].self, forKey: .vaultProviders)) ?? []
        providers = (try? container.decode([String].self, forKey: .providers)) ?? []
        providerUpstreams = (try? container.decode([String: String].self, forKey: .providerUpstreams)) ?? [:]
        rateLimits = (try? container.decode([String: Int].self, forKey: .rateLimits)) ?? [:]
        killSwitchActive = (try? container.decode(Bool.self, forKey: .killSwitchActive)) ?? false
        lastRequestAt = try? container.decode(String.self, forKey: .lastRequestAt)
        warnings = (try? container.decode([String].self, forKey: .warnings)) ?? []
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
            providers: [],
            providerUpstreams: [:],
            rateLimits: [:],
            killSwitchActive: false,
            lastRequestAt: nil,
            warnings: []
        )
    }

    init(
        generatedAt: Date,
        running: Bool,
        pid: Int?,
        addr: String?,
        todayRequests: Int,
        todayInputTokens: Int64,
        todayOutputTokens: Int64,
        todayCostCents: Int64,
        topProvider: String?,
        topProviderCents: Int64?,
        budgets: [TokfenceBudget],
        revokedProviders: [String],
        vaultProviders: [String],
        providers: [String],
        providerUpstreams: [String: String],
        rateLimits: [String: Int],
        killSwitchActive: Bool,
        lastRequestAt: String?,
        warnings: [String]
    ) {
        self.generatedAt = generatedAt
        self.running = running
        self.pid = pid
        self.addr = addr
        self.todayRequests = todayRequests
        self.todayInputTokens = todayInputTokens
        self.todayOutputTokens = todayOutputTokens
        self.todayCostCents = todayCostCents
        self.topProvider = topProvider
        self.topProviderCents = topProviderCents
        self.budgets = budgets
        self.revokedProviders = revokedProviders
        self.vaultProviders = vaultProviders
        self.providers = providers
        self.providerUpstreams = providerUpstreams
        self.rateLimits = rateLimits
        self.killSwitchActive = killSwitchActive
        self.lastRequestAt = lastRequestAt
        self.warnings = warnings
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
