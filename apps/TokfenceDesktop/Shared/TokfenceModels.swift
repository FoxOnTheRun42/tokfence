import Foundation

enum TokfenceSection: String, Identifiable {
    case agents
    case overview
    case vault
    case activity
    case budget
    case providers
    case settings

    var id: String { rawValue }

    var title: String {
        switch self {
        case .agents:
            return "Agents"
        case .overview:
            return "Overview"
        case .vault:
            return "Vault"
        case .activity:
            return "Activity"
        case .budget:
            return "Budget"
        case .providers:
            return "Providers"
        case .settings:
            return "Settings"
        }
    }

    var symbol: String {
        switch self {
        case .agents:
            return "cpu.fill"
        case .overview:
            return "gauge.with.dots.needle.33percent"
        case .vault:
            return "key.fill"
        case .activity:
            return "list.bullet.rectangle"
        case .budget:
            return "dollarsign.circle"
        case .providers:
            return "server.rack"
        case .settings:
            return "gearshape"
        }
    }

    /// Primary zone: the main feature (Agents)
    static let primary: [TokfenceSection] = [.agents]

    /// Secondary zone: proxy infrastructure
    static let proxy: [TokfenceSection] = [.overview, .vault, .activity, .budget, .providers]

    /// Utility zone
    static let utility: [TokfenceSection] = [.settings]
}

enum TokfenceAgentStatus: String, Hashable {
    case stopped
    case starting
    case running
    case error
    case placeholder
}

struct TokfenceAgentCardModel: Identifiable, Hashable {
    let id: String
    let name: String
    let subtitle: String
    let status: TokfenceAgentStatus
    let uptimeText: String
    let gatewayURL: String
    let dashboardURL: String
    let providers: [String]
    let recentActivity: [TokfenceLogRecord]
    let lastError: String
    let isPlaceholder: Bool
}

struct TokfenceDaemonStatus: Codable, Hashable {
    let running: Bool
    let pid: Int?
    let addr: String?
    let started: Date?
    let error: String?
}

struct TokfenceLogRecord: Codable, Hashable, Identifiable {
    let id: String
    let timestamp: Date
    let provider: String
    let model: String
    let endpoint: String
    let method: String
    let inputTokens: Int64
    let outputTokens: Int64
    let cacheReadTokens: Int64
    let cacheCreationTokens: Int64
    let estimatedCostCents: Int64
    let statusCode: Int
    let latencyMS: Int
    let ttftMS: Int
    let callerPID: Int
    let callerName: String
    let isStreaming: Bool
    let errorType: String
    let errorMessage: String
    let requestHash: String

    enum PascalKeys: String, CodingKey {
        case id = "ID"
        case timestamp = "Timestamp"
        case provider = "Provider"
        case model = "Model"
        case endpoint = "Endpoint"
        case method = "Method"
        case inputTokens = "InputTokens"
        case outputTokens = "OutputTokens"
        case cacheReadTokens = "CacheReadTokens"
        case cacheCreationTokens = "CacheCreationTokens"
        case estimatedCostCents = "EstimatedCostCents"
        case statusCode = "StatusCode"
        case latencyMS = "LatencyMS"
        case ttftMS = "TTFTMS"
        case callerPID = "CallerPID"
        case callerName = "CallerName"
        case isStreaming = "IsStreaming"
        case errorType = "ErrorType"
        case errorMessage = "ErrorMessage"
        case requestHash = "RequestHash"
    }

    enum SnakeKeys: String, CodingKey {
        case id
        case timestamp
        case provider
        case model
        case endpoint
        case method
        case inputTokens = "input_tokens"
        case outputTokens = "output_tokens"
        case cacheReadTokens = "cache_read_tokens"
        case cacheCreationTokens = "cache_creation_tokens"
        case estimatedCostCents = "estimated_cost_cents"
        case statusCode = "status_code"
        case latencyMS = "latency_ms"
        case ttftMS = "ttft_ms"
        case callerPID = "caller_pid"
        case callerName = "caller_name"
        case isStreaming = "is_streaming"
        case errorType = "error_type"
        case errorMessage = "error_message"
        case requestHash = "request_hash"
    }

    init(from decoder: Decoder) throws {
        if let c = try? decoder.container(keyedBy: SnakeKeys.self), c.contains(.id) {
            id = (try? c.decode(String.self, forKey: .id)) ?? ""
            timestamp = (try? c.decode(Date.self, forKey: .timestamp)) ?? Date.distantPast
            provider = (try? c.decode(String.self, forKey: .provider)) ?? ""
            model = (try? c.decode(String.self, forKey: .model)) ?? ""
            endpoint = (try? c.decode(String.self, forKey: .endpoint)) ?? ""
            method = (try? c.decode(String.self, forKey: .method)) ?? "POST"
            inputTokens = (try? c.decode(Int64.self, forKey: .inputTokens)) ?? 0
            outputTokens = (try? c.decode(Int64.self, forKey: .outputTokens)) ?? 0
            cacheReadTokens = (try? c.decode(Int64.self, forKey: .cacheReadTokens)) ?? 0
            cacheCreationTokens = (try? c.decode(Int64.self, forKey: .cacheCreationTokens)) ?? 0
            estimatedCostCents = (try? c.decode(Int64.self, forKey: .estimatedCostCents)) ?? 0
            statusCode = (try? c.decode(Int.self, forKey: .statusCode)) ?? 0
            latencyMS = (try? c.decode(Int.self, forKey: .latencyMS)) ?? 0
            ttftMS = (try? c.decode(Int.self, forKey: .ttftMS)) ?? 0
            callerPID = (try? c.decode(Int.self, forKey: .callerPID)) ?? 0
            callerName = (try? c.decode(String.self, forKey: .callerName)) ?? ""
            isStreaming = (try? c.decode(Bool.self, forKey: .isStreaming)) ?? false
            errorType = (try? c.decode(String.self, forKey: .errorType)) ?? ""
            errorMessage = (try? c.decode(String.self, forKey: .errorMessage)) ?? ""
            requestHash = (try? c.decode(String.self, forKey: .requestHash)) ?? ""
            return
        }

        let c = try decoder.container(keyedBy: PascalKeys.self)
        id = (try? c.decode(String.self, forKey: .id)) ?? ""
        timestamp = (try? c.decode(Date.self, forKey: .timestamp)) ?? Date.distantPast
        provider = (try? c.decode(String.self, forKey: .provider)) ?? ""
        model = (try? c.decode(String.self, forKey: .model)) ?? ""
        endpoint = (try? c.decode(String.self, forKey: .endpoint)) ?? ""
        method = (try? c.decode(String.self, forKey: .method)) ?? "POST"
        inputTokens = (try? c.decode(Int64.self, forKey: .inputTokens)) ?? 0
        outputTokens = (try? c.decode(Int64.self, forKey: .outputTokens)) ?? 0
        cacheReadTokens = (try? c.decode(Int64.self, forKey: .cacheReadTokens)) ?? 0
        cacheCreationTokens = (try? c.decode(Int64.self, forKey: .cacheCreationTokens)) ?? 0
        estimatedCostCents = (try? c.decode(Int64.self, forKey: .estimatedCostCents)) ?? 0
        statusCode = (try? c.decode(Int.self, forKey: .statusCode)) ?? 0
        latencyMS = (try? c.decode(Int.self, forKey: .latencyMS)) ?? 0
        ttftMS = (try? c.decode(Int.self, forKey: .ttftMS)) ?? 0
        callerPID = (try? c.decode(Int.self, forKey: .callerPID)) ?? 0
        callerName = (try? c.decode(String.self, forKey: .callerName)) ?? ""
        isStreaming = (try? c.decode(Bool.self, forKey: .isStreaming)) ?? false
        errorType = (try? c.decode(String.self, forKey: .errorType)) ?? ""
        errorMessage = (try? c.decode(String.self, forKey: .errorMessage)) ?? ""
        requestHash = (try? c.decode(String.self, forKey: .requestHash)) ?? ""
    }
}

struct TokfenceStatsRow: Codable, Hashable, Identifiable {
    let group: String
    let requestCount: Int
    let inputTokens: Int64
    let outputTokens: Int64
    let estimatedCostCents: Int64

    var id: String { group }

    enum PascalKeys: String, CodingKey {
        case group = "Group"
        case requestCount = "RequestCount"
        case inputTokens = "InputTokens"
        case outputTokens = "OutputTokens"
        case estimatedCostCents = "EstimatedCostCents"
    }

    enum SnakeKeys: String, CodingKey {
        case group
        case requestCount = "request_count"
        case inputTokens = "input_tokens"
        case outputTokens = "output_tokens"
        case estimatedCostCents = "estimated_cost_cents"
    }

    init(from decoder: Decoder) throws {
        if let c = try? decoder.container(keyedBy: SnakeKeys.self), c.contains(.group) {
            group = (try? c.decode(String.self, forKey: .group)) ?? ""
            requestCount = (try? c.decode(Int.self, forKey: .requestCount)) ?? 0
            inputTokens = (try? c.decode(Int64.self, forKey: .inputTokens)) ?? 0
            outputTokens = (try? c.decode(Int64.self, forKey: .outputTokens)) ?? 0
            estimatedCostCents = (try? c.decode(Int64.self, forKey: .estimatedCostCents)) ?? 0
            return
        }

        let c = try decoder.container(keyedBy: PascalKeys.self)
        group = (try? c.decode(String.self, forKey: .group)) ?? ""
        requestCount = (try? c.decode(Int.self, forKey: .requestCount)) ?? 0
        inputTokens = (try? c.decode(Int64.self, forKey: .inputTokens)) ?? 0
        outputTokens = (try? c.decode(Int64.self, forKey: .outputTokens)) ?? 0
        estimatedCostCents = (try? c.decode(Int64.self, forKey: .estimatedCostCents)) ?? 0
    }
}

struct TokfenceVaultProvidersResponse: Codable, Hashable {
    let providers: [String]
}

struct TokfenceCommandResult: Codable, Hashable {
    let status: String?
}

struct TokfenceLaunchResult: Codable, Hashable {
    let containerID: String
    let gatewayURL: String
    let gatewayToken: String
    let dashboardURL: String
    let providers: [String]
    let primaryModel: String
    let configPath: String
    let status: String
    let logsPreview: String?

    var isRunning: Bool {
        status.lowercased() == "running"
    }

    init(
        containerID: String = "",
        gatewayURL: String = "",
        gatewayToken: String = "",
        dashboardURL: String = "",
        providers: [String] = [],
        primaryModel: String = "",
        configPath: String = "",
        status: String = "",
        logsPreview: String? = nil
    ) {
        self.containerID = containerID
        self.gatewayURL = gatewayURL
        self.gatewayToken = gatewayToken
        self.dashboardURL = dashboardURL
        self.providers = providers
        self.primaryModel = primaryModel
        self.configPath = configPath
        self.status = status
        self.logsPreview = logsPreview
    }

    init(from decoder: Decoder) throws {
        if let c = try? decoder.container(keyedBy: SnakeKeys.self), c.contains(.containerID) {
            containerID = (try? c.decode(String.self, forKey: .containerID)) ?? ""
            gatewayURL = (try? c.decode(String.self, forKey: .gatewayURL)) ?? ""
            gatewayToken = (try? c.decode(String.self, forKey: .gatewayToken)) ?? ""
            dashboardURL = (try? c.decode(String.self, forKey: .dashboardURL)) ?? ""
            providers = (try? c.decode([String].self, forKey: .providers)) ?? []
            primaryModel = (try? c.decode(String.self, forKey: .primaryModel)) ?? ""
            configPath = (try? c.decode(String.self, forKey: .configPath)) ?? ""
            status = (try? c.decode(String.self, forKey: .status)) ?? ""
            logsPreview = try? c.decode(String.self, forKey: .logsPreview)
            return
        }

        let c = try decoder.container(keyedBy: PascalKeys.self)
        containerID = (try? c.decode(String.self, forKey: .containerID)) ?? ""
        gatewayURL = (try? c.decode(String.self, forKey: .gatewayURL)) ?? ""
        gatewayToken = (try? c.decode(String.self, forKey: .gatewayToken)) ?? ""
        dashboardURL = (try? c.decode(String.self, forKey: .dashboardURL)) ?? ""
        providers = (try? c.decode([String].self, forKey: .providers)) ?? []
        primaryModel = (try? c.decode(String.self, forKey: .primaryModel)) ?? ""
        configPath = (try? c.decode(String.self, forKey: .configPath)) ?? ""
        status = (try? c.decode(String.self, forKey: .status)) ?? ""
        logsPreview = try? c.decode(String.self, forKey: .logsPreview)
    }

    enum PascalKeys: String, CodingKey {
        case containerID = "ContainerID"
        case gatewayURL = "GatewayURL"
        case gatewayToken = "GatewayToken"
        case dashboardURL = "DashboardURL"
        case providers = "Providers"
        case primaryModel = "PrimaryModel"
        case configPath = "ConfigPath"
        case status = "Status"
        case logsPreview = "LogsPreview"
    }

    enum SnakeKeys: String, CodingKey {
        case containerID = "container_id"
        case gatewayURL = "gateway_url"
        case gatewayToken = "gateway_token"
        case dashboardURL = "dashboard_url"
        case providers
        case primaryModel = "primary_model"
        case configPath = "config_path"
        case status
        case logsPreview = "logs_preview"
    }
}
