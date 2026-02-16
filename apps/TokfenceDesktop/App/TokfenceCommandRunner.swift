import Foundation

struct TokfenceCommandRunner {
    private let binaryPathKey = "tokfence.desktop.binaryPath"
    private let commandTimeout: TimeInterval = 8.0
    private let retryAttempts = 2

    var configuredBinaryPath: String {
        get {
            if let stored = UserDefaults.standard.string(forKey: binaryPathKey), !stored.isEmpty {
                return stored
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

    func rotateVaultKey(provider: String, key: String) throws {
        _ = try run(arguments: ["vault", "rotate", provider, "-"], stdin: key + "\n")
    }

    func removeVaultKey(provider: String) throws {
        _ = try run(arguments: ["vault", "remove", provider])
    }

    func shellSnippet() throws -> String {
        try run(arguments: ["env"])
    }

    func runAction(arguments: [String]) throws {
        _ = try run(arguments: arguments)
    }

    private func runJSON<T: Decodable>(_ arguments: [String], decode type: T.Type) throws -> T {
        var lastError: Error?
        for attempt in 0..<retryAttempts {
            do {
                let output = try run(arguments: arguments)
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
                return try JSONDecoder.tokfence.decode(T.self, from: data)
            } catch {
                lastError = error
                if attempt < retryAttempts-1 && isTransientCommandFailure(error) {
                    Thread.sleep(forTimeInterval: 0.15)
                    continue
                }
                throw NSError(
                    domain: "TokfenceCommandRunner",
                    code: 4,
                    userInfo: [
                        NSLocalizedDescriptionKey: "Failed to decode tokfence JSON output for \(arguments.joined(separator: " ")): \(error.localizedDescription)",
                    ],
                )
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

    private func run(arguments: [String], stdin: String? = nil) throws -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: configuredBinaryPath)
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
                NSLocalizedDescriptionKey: "Failed to start tokfence binary at \(configuredBinaryPath). Set a valid binary path in the app."
            ])
        }

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
            let message = stderr.isEmpty ? "tokfence command failed (exit \(process.terminationStatus))" : stderr
            throw NSError(domain: "TokfenceCommandRunner", code: Int(process.terminationStatus), userInfo: [NSLocalizedDescriptionKey: message])
        }

        return output
    }

    private func isTransientCommandFailure(_ error: Error) -> Bool {
        let message = error.localizedDescription.lowercased()
        return message.contains("timed out") || message.contains("connection refused") || message.contains("exit code 2")
    }

    private func resolveDefaultBinaryPath() -> String {
        let preferred = [
            "\(FileManager.default.homeDirectoryForCurrentUser.path)/tmp/glasbox/glasbox/bin/tokfence",
            "/opt/homebrew/bin/tokfence",
            "/usr/local/bin/tokfence"
        ]
        for path in preferred where FileManager.default.isExecutableFile(atPath: path) {
            return path
        }

        let whichProcess = Process()
        whichProcess.executableURL = URL(fileURLWithPath: "/usr/bin/which")
        whichProcess.arguments = ["tokfence"]
        let pipe = Pipe()
        whichProcess.standardOutput = pipe
        try? whichProcess.run()
        whichProcess.waitUntilExit()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        let detected = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        if !detected.isEmpty {
            return detected
        }
        return "/opt/homebrew/bin/tokfence"
    }
}
