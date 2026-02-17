import Foundation

struct TokfenceStreamingProbeResult {
    let provider: String
    let baseURL: String
    let statusCode: Int
    let contentType: String
    let streamChunkReceived: Bool
    let firstChunkMS: Int?
    let responsePreview: String
}

struct TokfenceCommandRunner {
    private let binaryPathKey = "tokfence.desktop.binaryPath"
    private let commandTimeout: TimeInterval = 8.0
    private let launchCommandTimeout: TimeInterval = 90.0
    private let retryAttempts = 2

    var configuredBinaryPath: String {
        get {
            if let stored = UserDefaults.standard.string(forKey: binaryPathKey), !stored.isEmpty {
                let expanded = expandPath(stored)
                if FileManager.default.isExecutableFile(atPath: expanded) {
                    return expanded
                }
            }
            return resolveDefaultBinaryPath()
        }
        set {
            UserDefaults.standard.set(newValue, forKey: binaryPathKey)
        }
    }

    func fetchSnapshot() throws -> TokfenceSnapshot {
        try runJSON(["widget", "render", "--json"], decode: TokfenceSnapshot.self)
    }

    func fetchStatus() throws -> TokfenceDaemonStatus {
        try runJSON(["status", "--json"], decode: TokfenceDaemonStatus.self)
    }

    func fetchBudgets() throws -> [TokfenceBudget] {
        try runJSON(["budget", "status", "--json"], decode: [TokfenceBudget].self)
    }

    func fetchLogs(provider: String? = nil, since: String? = nil, model: String? = nil) throws -> [TokfenceLogRecord] {
        var args = ["log", "--json"]
        if let provider, !provider.isEmpty {
            args += ["--provider", provider]
        }
        if let since, !since.isEmpty {
            args += ["--since", since]
        }
        if let model, !model.isEmpty {
            args += ["--model", model]
        }
        return try runJSON(args, decode: [TokfenceLogRecord].self)
    }

    func fetchLog(requestID: String) throws -> TokfenceLogRecord {
        try runJSON(["log", requestID, "--json"], decode: TokfenceLogRecord.self)
    }

    func fetchStats(period: String = "today", provider: String? = nil, by: String = "provider") throws -> [TokfenceStatsRow] {
        var args = ["stats", "--period", period, "--by", by, "--json"]
        if let provider, !provider.isEmpty {
            args += ["--provider", provider]
        }
        return try runJSON(args, decode: [TokfenceStatsRow].self)
    }

    func fetchVaultProviders() throws -> [String] {
        let response = try runJSON(["vault", "list", "--json"], decode: TokfenceVaultProvidersResponse.self)
        return response.providers
    }

    func fetchRateLimits() throws -> [String: Int] {
        do {
            return try runJSON(["ratelimit", "status", "--json"], decode: [String: Int].self)
        } catch {
            return [:]
        }
    }

    func fetchEnv(provider: String? = nil) throws -> [String: String] {
        var args = ["env", "--json"]
        if let provider, !provider.isEmpty {
            args += ["--provider", provider]
        }
        return try runJSON(args, decode: [String: String].self)
    }

    func launchStart(
        image: String? = nil,
        name: String? = nil,
        port: Int? = nil,
        workspace: String? = nil,
        noPull: Bool = false,
        noOpen: Bool = true,
        json: Bool = true
    ) throws -> TokfenceLaunchResult {
        var args = ["launch"]
        if json {
            args.append("--json")
        }
        if let image, !image.isEmpty {
            args += ["--image", image]
        }
        if let name, !name.isEmpty {
            args += ["--name", name]
        }
        if let port {
            args += ["--port", "\(port)"]
        }
        if let workspace, !workspace.isEmpty {
            args += ["--workspace", workspace]
        }
        if noPull {
            args.append("--no-pull")
        }
        if noOpen {
            args.append("--no-open")
        }
        return try runJSON(args, decode: TokfenceLaunchResult.self, timeout: launchCommandTimeout)
    }

    func launchStatus() throws -> TokfenceLaunchResult {
        try runJSON(["launch", "status", "--json"], decode: TokfenceLaunchResult.self)
    }

    func launchConfig() throws -> String {
        try run(arguments: ["launch", "config"])
    }

    func launchStop() throws {
        _ = try run(arguments: ["launch", "stop"])
    }

    func launchRestart() throws -> TokfenceLaunchResult {
        return try runJSON(["launch", "restart", "--json"], decode: TokfenceLaunchResult.self, timeout: launchCommandTimeout)
    }

    func launchLogs(follow: Bool = false) throws -> String {
        var args = ["launch", "logs"]
        if follow {
            args += ["-f"]
        }
        let timeout = follow ? commandTimeout : launchCommandTimeout
        return try run(arguments: args, timeout: timeout)
    }

    func startDaemon() throws {
        _ = try run(arguments: ["start", "-d"])
    }

    func stopDaemon() throws {
        _ = try run(arguments: ["stop"])
    }

    func killSwitchOn() throws {
        _ = try run(arguments: ["kill"])
    }

    func killSwitchOff() throws {
        _ = try run(arguments: ["unkill"])
    }

    func revokeProvider(_ provider: String) throws {
        _ = try run(arguments: ["revoke", provider])
    }

    func restoreProvider(_ provider: String) throws {
        _ = try run(arguments: ["restore", provider])
    }

    func setRateLimit(provider: String, rpm: Int) throws {
        _ = try run(arguments: ["ratelimit", "set", provider, "\(rpm)"])
    }

    func clearRateLimit(provider: String) throws {
        _ = try run(arguments: ["ratelimit", "clear", provider])
    }

    func setBudget(provider: String, amountUSD: Double, period: String) throws {
        _ = try run(arguments: ["budget", "set", provider, String(format: "%.2f", amountUSD), period])
    }

    func clearBudget(provider: String) throws {
        _ = try run(arguments: ["budget", "clear", provider])
    }

    func addVaultKey(provider: String, key: String) throws {
        _ = try run(arguments: ["vault", "add", provider, "-"], stdin: key + "\n")
    }

    func setProviderEndpoint(provider: String, endpoint: String) throws {
        _ = try run(arguments: ["provider", "set", provider, endpoint])
    }

    func rotateVaultKey(provider: String, key: String) throws {
        _ = try run(arguments: ["vault", "rotate", provider, "-"], stdin: key + "\n")
    }

    func removeVaultKey(provider: String) throws {
        _ = try run(arguments: ["vault", "remove", provider])
    }

    func shellSnippet() throws -> String {
        let snippet = try run(arguments: ["env"])
        return normalizeShellSnippet(snippet)
    }

    func runAction(arguments: [String]) throws {
        _ = try run(arguments: arguments)
    }

    func runStreamingProbe(provider rawProvider: String, daemonAddr: String, model: String?) async throws -> TokfenceStreamingProbeResult {
        let provider = rawProvider.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard !provider.isEmpty else {
            throw NSError(
                domain: "TokfenceCommandRunner",
                code: 8,
                userInfo: [NSLocalizedDescriptionKey: "provider is required for streaming probe"]
            )
        }

        guard provider != "google" else {
            throw NSError(
                domain: "TokfenceCommandRunner",
                code: 8,
                userInfo: [NSLocalizedDescriptionKey: "automatic streaming probe for google is not supported yet"]
            )
        }

        let hostPort = normalizeHostPort(daemonAddr)
        let baseURL = "http://\(hostPort)/\(provider)"
        let (path, bodyData) = try probeRequest(provider: provider, model: model)

        guard let url = URL(string: baseURL + path) else {
            throw NSError(
                domain: "TokfenceCommandRunner",
                code: 8,
                userInfo: [NSLocalizedDescriptionKey: "invalid probe URL for provider \(provider)"]
            )
        }

        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.timeoutInterval = 25
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("text/event-stream", forHTTPHeaderField: "Accept")
        request.httpBody = bodyData

        let startedAt = Date()
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = 25
        configuration.timeoutIntervalForResource = 25
        let session = URLSession(configuration: configuration)
        let (bytes, response) = try await session.bytes(for: request)

        guard let http = response as? HTTPURLResponse else {
            throw NSError(
                domain: "TokfenceCommandRunner",
                code: 8,
                userInfo: [NSLocalizedDescriptionKey: "streaming probe returned non-HTTP response"]
            )
        }

        var streamChunkReceived = false
        var firstChunkMS: Int?
        var previewLines: [String] = []
        var inspectedLines = 0

        for try await line in bytes.lines {
            inspectedLines += 1

            let trimmed = line.trimmingCharacters(in: .whitespacesAndNewlines)
            if !trimmed.isEmpty && previewLines.count < 6 {
                previewLines.append(trimmed)
            }

            if line.hasPrefix("data:") && !line.hasPrefix("data: [DONE]") {
                streamChunkReceived = true
                firstChunkMS = Int(Date().timeIntervalSince(startedAt) * 1000.0)
                break
            }

            if inspectedLines >= 160 {
                break
            }

            if http.statusCode >= 400 && previewLines.joined(separator: " ").count > 420 {
                break
            }
        }

        let preview = previewLines.joined(separator: "\n")
        return TokfenceStreamingProbeResult(
            provider: provider,
            baseURL: baseURL,
            statusCode: http.statusCode,
            contentType: http.value(forHTTPHeaderField: "Content-Type") ?? "",
            streamChunkReceived: streamChunkReceived,
            firstChunkMS: firstChunkMS,
            responsePreview: preview
        )
    }

    private func runJSON<T: Decodable>(_ arguments: [String], decode type: T.Type) throws -> T {
        try runJSON(arguments, decode: type, timeout: commandTimeout)
    }

    private func runJSON<T: Decodable>(_ arguments: [String], decode type: T.Type, timeout: TimeInterval) throws -> T {
        var lastError: Error?
        for attempt in 0..<retryAttempts {
            do {
                let output = try run(arguments: arguments, timeout: timeout)
                let trimmed = output.trimmingCharacters(in: .whitespacesAndNewlines)
                guard !trimmed.isEmpty else {
                    throw NSError(
                        domain: "TokfenceCommandRunner",
                        code: 1,
                        userInfo: [NSLocalizedDescriptionKey: "Command output was empty"]
                    )
                }
                guard trimmed.first == "{" || trimmed.first == "[" else {
                    throw NSError(
                        domain: "TokfenceCommandRunner",
                        code: 1,
                        userInfo: [NSLocalizedDescriptionKey: "Command output was not JSON"]
                    )
                }
                guard let data = trimmed.data(using: .utf8) else {
                    throw NSError(
                        domain: "TokfenceCommandRunner",
                        code: 1,
                        userInfo: [NSLocalizedDescriptionKey: "Unable to decode command output as UTF-8"]
                    )
                }
                do {
                    return try JSONDecoder.tokfence.decode(T.self, from: data)
                } catch {
                    throw NSError(
                        domain: "TokfenceCommandRunner",
                        code: 4,
                        userInfo: [
                            NSLocalizedDescriptionKey: "Failed to decode tokfence JSON output for \(arguments.joined(separator: " ")): \(error.localizedDescription)",
                        ],
                    )
                }
            } catch {
                lastError = error
                if attempt < retryAttempts-1 && isTransientCommandFailure(error) {
                    Thread.sleep(forTimeInterval: 0.15)
                    continue
                }
                throw error
            }
        }
        if let lastError {
            throw lastError
        }
        throw NSError(
            domain: "TokfenceCommandRunner",
            code: 1,
            userInfo: [NSLocalizedDescriptionKey: "tokfence command failed"]
        )
    }

    private func run(arguments: [String], stdin: String? = nil, timeout: TimeInterval? = nil) throws -> String {
        let process = Process()
        guard let executablePath = resolveExecutablePathForRun() else {
            throw NSError(domain: "TokfenceCommandRunner", code: 2, userInfo: [
                NSLocalizedDescriptionKey: "Failed to locate tokfence binary. Checked: \(binaryLookupCandidates().joined(separator: ", "))",
            ])
        }
        process.executableURL = URL(fileURLWithPath: executablePath)
        process.arguments = arguments

        let outputPipe = Pipe()
        let errorPipe = Pipe()
        process.standardOutput = outputPipe
        process.standardError = errorPipe
        let inputPipe = Pipe()
        if stdin != nil {
            process.standardInput = inputPipe
        }

        let done = DispatchSemaphore(value: 0)
        process.terminationHandler = { _ in done.signal() }
        do {
            try process.run()
            if let stdin {
                inputPipe.fileHandleForWriting.write(Data(stdin.utf8))
                inputPipe.fileHandleForWriting.closeFile()
            }
        } catch {
            throw NSError(domain: "TokfenceCommandRunner", code: 2, userInfo: [
                NSLocalizedDescriptionKey: "Failed to start tokfence binary at \(executablePath). Set a valid binary path in the app."
            ])
        }

        let commandTimeout = timeout ?? self.commandTimeout
        let timeout = DispatchTime.now() + commandTimeout
        if done.wait(timeout: timeout) == .timedOut {
            process.terminate()
            _ = done.wait(timeout: .now() + 1)
            throw NSError(domain: "TokfenceCommandRunner", code: 7, userInfo: [
                NSLocalizedDescriptionKey: "tokfence command timed out after \(Int(commandTimeout))s"
            ])
        }

        let outputData = outputPipe.fileHandleForReading.readDataToEndOfFile()
        let errorData = errorPipe.fileHandleForReading.readDataToEndOfFile()
        let output = String(decoding: outputData, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        let stderr = String(decoding: errorData, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)

        if process.terminationStatus != 0 {
            let message: String
            if stderr.isEmpty {
                message = "tokfence command failed (exit \(process.terminationStatus))"
            } else {
                message = sanitizeCLIError(stderr)
            }
            throw NSError(domain: "TokfenceCommandRunner", code: Int(process.terminationStatus), userInfo: [NSLocalizedDescriptionKey: message])
        }

        return output
    }

    private func isTransientCommandFailure(_ error: Error) -> Bool {
        let message = error.localizedDescription.lowercased()
        return message.contains("timed out") || message.contains("connection refused") || message.contains("exit code 2")
    }

    private func sanitizeCLIError(_ raw: String) -> String {
        let lines = raw
            .components(separatedBy: .newlines)
            .map { $0.trimmingCharacters(in: .whitespacesAndNewlines) }
            .filter { !$0.isEmpty }

        var cleaned: [String] = []
        for line in lines {
            let lower = line.lowercased()
            if lower.hasPrefix("usage:") || lower.hasPrefix("available commands:") || lower.hasPrefix("flags:") || lower.hasPrefix("global flags:") {
                break
            }
            if lower.hasPrefix("use \"tokfence launch") {
                break
            }
            if line.hasPrefix("✓") {
                continue
            }
            if lower.hasPrefix("error: ") {
                cleaned.append(String(line.dropFirst(7)).trimmingCharacters(in: .whitespaces))
                continue
            }
            if lower.hasPrefix("tokfence launch [") {
                continue
            }
            cleaned.append(line)
        }

        let unique = cleaned.reduce(into: [String]()) { acc, line in
            if !acc.contains(line) {
                acc.append(line)
            }
        }

        if unique.isEmpty {
            return lines.first ?? raw
        }
        return unique.joined(separator: "\n")
    }

    private func resolveDefaultBinaryPath() -> String {
        if let detected = resolveExecutablePathForRun() {
            return detected
        }
        return "/opt/homebrew/bin/tokfence"
    }

    private func resolveExecutablePathForRun() -> String? {
        for candidate in binaryLookupCandidates() {
            if FileManager.default.isExecutableFile(atPath: candidate) {
                if UserDefaults.standard.string(forKey: binaryPathKey) != candidate {
                    UserDefaults.standard.set(candidate, forKey: binaryPathKey)
                }
                return candidate
            }
        }
        return nil
    }

    private func binaryLookupCandidates() -> [String] {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        let cwd = FileManager.default.currentDirectoryPath
        var candidates: [String] = []

        if let stored = UserDefaults.standard.string(forKey: binaryPathKey), !stored.isEmpty {
            candidates.append(stored)
        }

        if let envBinary = ProcessInfo.processInfo.environment["TOKFENCE_BINARY"], !envBinary.isEmpty {
            candidates.append(envBinary)
        }

        candidates.append(contentsOf: [
            "\(home)/bin/tokfence",
            "\(home)/.local/bin/tokfence",
            "/opt/homebrew/bin/tokfence",
            "/usr/local/bin/tokfence",
            "/usr/bin/tokfence",
            "/tmp/tokfence",
            "\(cwd)/tokfence",
            "\(cwd)/bin/tokfence",
            "\(home)/tmp/glasbox/glasbox/tokfence",
            "\(home)/tmp/glasbox/glasbox/bin/tokfence",
        ])

        if let whichDetected = detectBinaryWithWhich() {
            candidates.append(whichDetected)
        }

        var unique: [String] = []
        var seen = Set<String>()
        for rawPath in candidates {
            let expanded = expandPath(rawPath)
            guard !expanded.isEmpty else { continue }
            if seen.insert(expanded).inserted {
                unique.append(expanded)
            }
        }
        return unique
    }

    private func detectBinaryWithWhich() -> String? {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/env")
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = Pipe()

        let home = FileManager.default.homeDirectoryForCurrentUser.path
        let existingPath = ProcessInfo.processInfo.environment["PATH"] ?? ""
        let extra = [
            "/opt/homebrew/bin",
            "/usr/local/bin",
            "/usr/bin",
            "/bin",
            "\(home)/bin",
            "\(home)/.local/bin",
        ].joined(separator: ":")
        let mergedPath = [existingPath, extra]
            .filter { !$0.isEmpty }
            .joined(separator: ":")

        var env = ProcessInfo.processInfo.environment
        env["PATH"] = mergedPath
        process.environment = env
        process.arguments = ["which", "tokfence"]

        do {
            try process.run()
            process.waitUntilExit()
        } catch {
            return nil
        }

        guard process.terminationStatus == 0 else {
            return nil
        }
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        let detected = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        if detected.isEmpty {
            return nil
        }
        return detected
    }

    private func expandPath(_ path: String) -> String {
        (path as NSString).expandingTildeInPath
    }

    private func normalizeHostPort(_ raw: String) -> String {
        let value = raw.trimmingCharacters(in: .whitespacesAndNewlines)
        if value.hasPrefix("http://") || value.hasPrefix("https://"), let url = URL(string: value), let host = url.host {
            if let port = url.port {
                return "\(host):\(port)"
            }
            return host
        }
        if value.contains("/") {
            return value.replacingOccurrences(of: "http://", with: "")
                .replacingOccurrences(of: "https://", with: "")
                .trimmingCharacters(in: CharacterSet(charactersIn: "/"))
        }
        return value
    }

    private func probeRequest(provider: String, model: String?) throws -> (path: String, bodyData: Data) {
        let selectedModel = model?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""

        if provider == "anthropic" {
            let payload: [String: Any] = [
                "model": selectedModel.isEmpty ? "claude-3-5-haiku-latest" : selectedModel,
                "max_tokens": 24,
                "stream": true,
                "messages": [
                    ["role": "user", "content": "Reply with the single word OK."]
                ]
            ]
            return ("/v1/messages", try encodeJSONObject(payload))
        }

        let defaultModel: String
        switch provider {
        case "openai":
            defaultModel = "gpt-4o-mini"
        case "groq":
            defaultModel = "llama-3.1-8b-instant"
        case "mistral":
            defaultModel = "mistral-small-latest"
        case "openrouter":
            defaultModel = "openai/gpt-4o-mini"
        default:
            defaultModel = "gpt-4o-mini"
        }

        let payload: [String: Any] = [
            "model": selectedModel.isEmpty ? defaultModel : selectedModel,
            "messages": [
                ["role": "user", "content": "Reply with the single word OK."]
            ],
            "max_tokens": 24,
            "stream": true
        ]
        return ("/v1/chat/completions", try encodeJSONObject(payload))
    }

    private func encodeJSONObject(_ object: [String: Any]) throws -> Data {
        do {
            return try JSONSerialization.data(withJSONObject: object, options: [])
        } catch {
            throw NSError(
                domain: "TokfenceCommandRunner",
                code: 8,
                userInfo: [NSLocalizedDescriptionKey: "failed to encode streaming probe payload: \(error.localizedDescription)"]
            )
        }
    }

    private func normalizeShellSnippet(_ snippet: String) -> String {
        snippet
            .split(whereSeparator: \.isNewline)
            .map { line in
                let text = String(line)
                if text.hasPrefix("export "), let idx = text.firstIndex(of: "=") {
                    let keyStart = text.index(text.startIndex, offsetBy: 7)
                    let key = text[keyStart..<idx]
                        .replacingOccurrences(of: " ", with: "_")
                    let value = text[idx...]
                    return "export \(key)\(value)"
                }
                if text.hasPrefix("set -x ") {
                    let parts = text.split(separator: " ", maxSplits: 3, omittingEmptySubsequences: true)
                    if parts.count >= 3 {
                        let normalized = parts[2].replacingOccurrences(of: " ", with: "_")
                        if parts.count == 4 {
                            return "set -x \(normalized) \(parts[3])"
                        }
                        return "set -x \(normalized)"
                    }
                }
                return text
            }
            .joined(separator: "\n")
    }
}
