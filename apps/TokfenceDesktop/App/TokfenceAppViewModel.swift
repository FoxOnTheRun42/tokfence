import AppKit
import Foundation
import SwiftUI
@preconcurrency import UserNotifications
import WidgetKit

enum TokfenceLogStatusFilter: String, CaseIterable, Identifiable {
    case all
    case success
    case rateLimited
    case errors

    var id: String { rawValue }

    var title: String {
        switch self {
        case .all:
            return "All"
        case .success:
            return "Success (2xx)"
        case .rateLimited:
            return "Rate Limited (429)"
        case .errors:
            return "Error (4xx/5xx)"
        }
    }
}

enum TokfenceTimeRange: String, CaseIterable, Identifiable {
    case oneHour = "1h"
    case sixHours = "6h"
    case twentyFourHours = "24h"
    case sevenDays = "7d"

    var id: String { rawValue }
}

struct TokfenceHourlyBucket: Identifiable, Hashable {
    let hourStart: Date
    var counts: [String: Int]

    var id: Date { hourStart }

    var totalCount: Int {
        counts.values.reduce(0, +)
    }
}

struct TokfenceProviderOverview: Identifiable, Hashable {
    let provider: String
    let upstream: String
    let hasKey: Bool
    let isRevoked: Bool
    let rateLimitRPM: Int?
    let todayRequests: Int
    let todayInputTokens: Int64
    let todayOutputTokens: Int64
    let todayCostCents: Int64

    var id: String { provider }
}

struct TokfenceSetupWizardResult {
    let provider: String
    let baseURL: String
    let daemonReachable: Bool
    let keyStored: Bool
    let probe: TokfenceStreamingProbeResult
    let logRecord: TokfenceLogRecord?

    var logFound: Bool {
        logRecord != nil
    }

    var tokensLogged: Bool {
        guard let logRecord else { return false }
        return (logRecord.inputTokens + logRecord.outputTokens) > 0
    }

    var costLogged: Bool {
        guard let logRecord else { return false }
        return logRecord.estimatedCostCents > 0
    }
}

@MainActor
final class TokfenceAppViewModel: ObservableObject {
    private enum BudgetAlertLevel: Int {
        case none = 0
        case warning = 1
        case exceeded = 2
    }

    @Published var selectedSection: TokfenceSection = .dashboard
    @Published var snapshot: TokfenceSnapshot = TokfenceSharedStore.loadSnapshot()
    @Published var daemonStatus: TokfenceDaemonStatus = TokfenceDaemonStatus(running: false, pid: nil, addr: nil, started: nil, error: nil)
    @Published var logs: [TokfenceLogRecord] = []
    @Published var statsByProvider: [TokfenceStatsRow] = []
    @Published var statsByModel: [TokfenceStatsRow] = []
    @Published var hourlyBuckets: [TokfenceHourlyBucket] = []
    @Published var envMap: [String: String] = [:]
    @Published var isRefreshing = false
    @Published var lastError = ""
    @Published var showErrorToast = false
    @Published var daemonIdentityMismatchError = ""
    @Published var showDaemonIdentityMismatchDialog = false
    @Published var binaryPath: String = ""
    @Published var liveTailEnabled = true
    @Published var logProviderFilter = "all"
    @Published var logStatusFilter: TokfenceLogStatusFilter = .all
    @Published var logTimeRange: TokfenceTimeRange = .twentyFourHours
    @Published var logQuery = ""
    @Published var launchResult = TokfenceLaunchResult()
    @Published var launchBusy = false
    @Published var launchLogsOutput = ""
    @Published var launchConfigOutput = ""

    private var runner = TokfenceCommandRunner()
    private var refreshTask: Task<Void, Never>?
    private var tickCount = 0
    private var refreshTaskInProgress = false
    private var logsRefreshInProgress = false
    private var toastDismissTask: Task<Void, Never>?
    private var lastBudgetAlertLevel: BudgetAlertLevel = .none
    private var lastBudgetAlertDay = ""

    private let budgetWarningThreshold = 0.8
    private let budgetAlertLevelDefaultsKey = "TokfenceDesktop.lastBudgetAlertLevel"
    private let budgetAlertDayDefaultsKey = "TokfenceDesktop.lastBudgetAlertDay"
    private let preferredModelDefaultsPrefix = "TokfenceDesktop.preferredModel."

    init() {
        binaryPath = runner.configuredBinaryPath
        let defaults = UserDefaults.standard
        let rawLevel = defaults.integer(forKey: budgetAlertLevelDefaultsKey)
        lastBudgetAlertLevel = BudgetAlertLevel(rawValue: rawLevel) ?? .none
        lastBudgetAlertDay = defaults.string(forKey: budgetAlertDayDefaultsKey) ?? ""
    }

    var providers: [String] {
        if !snapshot.providers.isEmpty {
            return snapshot.providers.sorted()
        }
        let fallback = Set(snapshot.vaultProviders + logs.map(\.provider))
        return fallback.sorted()
    }

    var filteredLogs: [TokfenceLogRecord] {
        logs.filter { record in
            if logProviderFilter != "all", record.provider != logProviderFilter {
                return false
            }
            switch logStatusFilter {
            case .all:
                break
            case .success:
                if record.statusCode < 200 || record.statusCode >= 300 {
                    return false
                }
            case .rateLimited:
                if record.statusCode != 429 {
                    return false
                }
            case .errors:
                if record.statusCode < 400 {
                    return false
                }
            }
            if !logQuery.isEmpty {
                let query = logQuery.lowercased()
                let haystack = [
                    record.provider.lowercased(),
                    record.model.lowercased(),
                    record.id.lowercased(),
                    record.callerName.lowercased()
                ].joined(separator: " ")
                if !haystack.contains(query) {
                    return false
                }
            }
            return true
        }
    }

    var providerOverview: [TokfenceProviderOverview] {
        let statsMap = Dictionary(uniqueKeysWithValues: statsByProvider.map { ($0.group.lowercased(), $0) })
        let revoked = Set(snapshot.revokedProviders)
        let vault = Set(snapshot.vaultProviders)
        let providersToShow = providers

        return providersToShow.map { provider in
            let stats = statsMap[provider]
            return TokfenceProviderOverview(
                provider: provider,
                upstream: snapshot.providerUpstreams[provider] ?? "n/a",
                hasKey: vault.contains(provider),
                isRevoked: revoked.contains(provider),
                rateLimitRPM: snapshot.rateLimits[provider],
                todayRequests: stats?.requestCount ?? 0,
                todayInputTokens: stats?.inputTokens ?? 0,
                todayOutputTokens: stats?.outputTokens ?? 0,
                todayCostCents: stats?.estimatedCostCents ?? 0
            )
        }
    }

    var globalDailyBudget: TokfenceBudget? {
        snapshot.budgets.first(where: { $0.provider == "global" && $0.period.lowercased() == "daily" })
    }

    var globalMonthlyBudget: TokfenceBudget? {
        snapshot.budgets.first(where: { $0.provider == "global" && $0.period.lowercased() == "monthly" })
    }

    func start() {
        refreshTask?.cancel()
        refreshTask = Task { [weak self] in
            guard let self else { return }
            await self.refreshAll()
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(4))
                self.tickCount += 1
                if self.liveTailEnabled {
                    await self.refreshLogsOnly()
                }
                if self.tickCount % 5 == 0 {
                    await self.refreshAll()
                }
            }
        }
    }

    func stop() {
        refreshTask?.cancel()
        refreshTask = nil
    }

    func saveBinaryPath() {
        runner.configuredBinaryPath = binaryPath
        Task { await refreshAll() }
    }

    func refreshAll() async {
        if refreshTaskInProgress {
            return
        }
        refreshTaskInProgress = true
        isRefreshing = true
        defer {
            refreshTaskInProgress = false
            isRefreshing = false
        }
        do {
            let previousGlobalBudget = self.globalDailyBudget
            let snapshot = try runner.fetchSnapshot()
            let status = try runner.fetchStatus()
            let logs = try runner.fetchLogs(since: logTimeRange.rawValue)
            let statsByProvider = try runner.fetchStats(period: "today", by: "provider")
            let statsByModel = try runner.fetchStats(period: "today", by: "model")
            let env = try runner.fetchEnv()

            self.snapshot = snapshot
            self.daemonStatus = status
            self.updateDaemonIdentityWarning(status)
            self.logs = logs.sorted(by: { $0.timestamp > $1.timestamp })
            self.statsByProvider = statsByProvider
            self.statsByModel = statsByModel
            self.envMap = env
            self.hourlyBuckets = try await loadHourlyBuckets()
            TokfenceSharedStore.saveSnapshot(snapshot)
            WidgetCenter.shared.reloadAllTimelines()
            maybeNotifyBudgetThreshold(previous: previousGlobalBudget, current: self.globalDailyBudget)
            clearError()
        } catch {
            setError(error.localizedDescription)
        }
    }

    func updateDaemonIdentityWarning(_ status: TokfenceDaemonStatus) {
        let error = (status.error ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        if status.running {
            if !error.isEmpty {
                setDaemonIdentityMismatch(message: error)
                return
            }
            clearDaemonIdentityMismatch()
            return
        }
        if isDaemonIdentityMismatchError(error) {
            setDaemonIdentityMismatch(message: error)
            return
        }
        clearDaemonIdentityMismatch()
    }

    func setDaemonIdentityMismatch(message: String) {
        daemonIdentityMismatchError = message
        showDaemonIdentityMismatchDialog = true
    }

    func clearDaemonIdentityMismatch() {
        daemonIdentityMismatchError = ""
        showDaemonIdentityMismatchDialog = false
    }

    func dismissDaemonIdentityMismatch() {
        showDaemonIdentityMismatchDialog = false
    }

    func isDaemonIdentityMismatchError(_ message: String) -> Bool {
        if message.isEmpty {
            return false
        }
        let lower = message.lowercased()
        return lower.contains("identity mismatch") || lower.contains("pid file owner mismatch")
    }

    func clearDaemonPIDFile() async {
        do {
            let pidPath = FileManager.default.homeDirectoryForCurrentUser
                .appendingPathComponent(".tokfence/tokfence.pid")
            if FileManager.default.fileExists(atPath: pidPath.path) {
                try FileManager.default.removeItem(at: pidPath)
            }
            clearDaemonIdentityMismatch()
            await refreshAll()
        } catch {
            setError("failed to clear daemon pid file: \(error.localizedDescription)")
        }
    }

    func refreshLogsOnly() async {
        if refreshTaskInProgress || logsRefreshInProgress {
            return
        }
        logsRefreshInProgress = true
        defer { logsRefreshInProgress = false }
        do {
            let logs = try runner.fetchLogs(since: logTimeRange.rawValue)
            self.logs = logs.sorted(by: { $0.timestamp > $1.timestamp })
            clearError()
        } catch {
            setError(error.localizedDescription)
        }
    }

    func loadLogDetail(requestID: String) -> TokfenceLogRecord? {
        do {
            return try runner.fetchLog(requestID: requestID)
        } catch {
            setError(error.localizedDescription)
            return nil
        }
    }

    func startDaemon() async { await runAndRefresh { try runner.startDaemon() } }
    func stopDaemon() async { await runAndRefresh { try runner.stopDaemon() } }
    func killSwitchOn() async { await runAndRefresh { try runner.killSwitchOn() } }
    func killSwitchOff() async { await runAndRefresh { try runner.killSwitchOff() } }

    func revokeProvider(_ provider: String) async { await runAndRefresh { try runner.revokeProvider(provider) } }
    func restoreProvider(_ provider: String) async { await runAndRefresh { try runner.restoreProvider(provider) } }

    func setRateLimit(provider: String, rpm: Int) async {
        await runAndRefresh { try runner.setRateLimit(provider: provider, rpm: rpm) }
    }

    func clearRateLimit(provider: String) async {
        await runAndRefresh { try runner.clearRateLimit(provider: provider) }
    }

    func launchStart(
        image: String? = nil,
        name: String? = nil,
        portText: String = "",
        workspace: String? = nil,
        noPull: Bool = false,
        openDashboard: Bool = true
    ) async {
        await runAndRefreshLaunch {
            let parsedPort = try parseLaunchPort(portText)
            let result = try runner.launchStart(
                image: image,
                name: name,
                port: parsedPort,
                workspace: workspace,
                noPull: noPull,
                noOpen: !openDashboard
            )
            launchResult = result
            launchLogsOutput = ""
            launchConfigOutput = ""
        }
    }

    func launchStatus() async {
        await runAndRefreshLaunch {
            let status = try runner.launchStatus()
            launchResult = status
        }
    }

    func launchConfig() async {
        await runAndRefresh {
            let output = try runner.launchConfig()
            launchConfigOutput = output
        }
    }

    func launchStop() async {
        await runAndRefreshLaunch {
            try runner.launchStop()
            launchResult = TokfenceLaunchResult(status: "stopped")
        }
    }

    func launchRestart() async {
        await runAndRefreshLaunch {
            let result = try runner.launchRestart()
            launchResult = result
            launchLogsOutput = ""
            launchConfigOutput = ""
        }
    }

    func launchLogs(follow: Bool = false) async {
        await runAndRefresh {
            let output = try runner.launchLogs(follow: follow)
            launchLogsOutput = output
        }
    }

    func setBudget(provider: String, amountUSD: Double, period: String) async {
        await runAndRefresh { try runner.setBudget(provider: provider, amountUSD: amountUSD, period: period) }
    }

    func clearBudget(provider: String) async {
        await runAndRefresh { try runner.clearBudget(provider: provider) }
    }

    func addVaultKey(provider: String, key: String, endpoint: String?, preferredModel: String?) async {
        await runAndRefresh {
            let normalizedProvider = provider.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
            guard !normalizedProvider.isEmpty else {
                throw NSError(
                    domain: "TokfenceDesktop",
                    code: 1,
                    userInfo: [NSLocalizedDescriptionKey: "Provider name is required"]
                )
            }

            let trimmedEndpoint = endpoint?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            if !trimmedEndpoint.isEmpty {
                try runner.setProviderEndpoint(provider: normalizedProvider, endpoint: trimmedEndpoint)
            }
            try runner.addVaultKey(provider: normalizedProvider, key: key)

            let trimmedModel = preferredModel?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
            if !trimmedModel.isEmpty {
                UserDefaults.standard.set(trimmedModel, forKey: preferredModelDefaultsPrefix + normalizedProvider)
            }
        }
    }

    func preferredModel(for provider: String) -> String? {
        let normalizedProvider = provider.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard !normalizedProvider.isEmpty else { return nil }
        let stored = UserDefaults.standard.string(forKey: preferredModelDefaultsPrefix + normalizedProvider)
        let trimmed = stored?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? nil : trimmed
    }

    func rotateVaultKey(provider: String, key: String) async {
        await runAndRefresh { try runner.rotateVaultKey(provider: provider, key: key) }
    }

    func removeVaultKey(provider: String) async {
        await runAndRefresh { try runner.removeVaultKey(provider: provider) }
    }

    func shellSnippet() -> String {
        (try? runner.shellSnippet()) ?? "eval \"$(tokfence env)\""
    }

    func runSetupWizard(provider rawProvider: String, endpoint rawEndpoint: String?, key: String, model: String?) async throws -> TokfenceSetupWizardResult {
        let provider = rawProvider.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        guard !provider.isEmpty else {
            throw NSError(
                domain: "TokfenceDesktop",
                code: 1,
                userInfo: [NSLocalizedDescriptionKey: "Provider name is required"]
            )
        }
        let keyTrimmed = key.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !keyTrimmed.isEmpty else {
            throw NSError(
                domain: "TokfenceDesktop",
                code: 1,
                userInfo: [NSLocalizedDescriptionKey: "API key is required"]
            )
        }

        var status = try runner.fetchStatus()
        if !status.running {
            try runner.startDaemon()
            let deadline = Date().addingTimeInterval(8)
            while Date() < deadline {
                try await Task.sleep(nanoseconds: 350_000_000)
                status = try runner.fetchStatus()
                if status.running {
                    break
                }
            }
        }
        guard status.running else {
            throw NSError(
                domain: "TokfenceDesktop",
                code: 1,
                userInfo: [NSLocalizedDescriptionKey: "Daemon did not become reachable in time"]
            )
        }
        let daemonAddr = status.addr ?? snapshot.addr ?? "127.0.0.1:9471"

        let endpoint = rawEndpoint?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if !endpoint.isEmpty {
            try runner.setProviderEndpoint(provider: provider, endpoint: endpoint)
        }

        try runner.addVaultKey(provider: provider, key: keyTrimmed)

        let existingLogs = try runner.fetchLogs(provider: provider, since: "1h")
        let baselineIDs = Set(existingLogs.map(\.id))

        let probe = try await runner.runStreamingProbe(provider: provider, daemonAddr: daemonAddr, model: model)

        var newLog: TokfenceLogRecord?
        let logDeadline = Date().addingTimeInterval(8)
        while Date() < logDeadline {
            let latest = try runner.fetchLogs(provider: provider, since: "1h")
            if let match = latest.first(where: { !baselineIDs.contains($0.id) }) {
                newLog = match
                break
            }
            try await Task.sleep(nanoseconds: 350_000_000)
        }

        await refreshAll()

        return TokfenceSetupWizardResult(
            provider: provider,
            baseURL: probe.baseURL,
            daemonReachable: true,
            keyStored: true,
            probe: probe,
            logRecord: newLog
        )
    }

    func openDataFolder() {
        let url = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".tokfence")
        NSWorkspace.shared.open(url)
    }

    private func runAndRefresh(_ operation: () throws -> Void) async {
        do {
            clearError()
            try operation()
            await refreshAll()
        } catch {
            setError(error.localizedDescription)
        }
    }

    private func runAndRefreshLaunch(_ operation: () throws -> Void) async {
        guard !launchBusy else {
            return
        }
        launchBusy = true
        clearError()
        defer {
            launchBusy = false
        }
        await runAndRefresh(operation)
    }

    private func parseLaunchPort(_ value: String) throws -> Int? {
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else {
            return nil
        }
        guard let port = Int(trimmed), port > 0 && port < 65_535 else {
            throw NSError(
                domain: "TokfenceDesktop",
                code: 1,
                userInfo: [NSLocalizedDescriptionKey: "Gateway port must be a valid integer between 1 and 65534"]
            )
        }
        return port
    }

    func dismissErrorToast() {
        withAnimation(TokfenceTheme.uiAnimation) {
            showErrorToast = false
        }
    }

    private func loadHourlyBuckets() async throws -> [TokfenceHourlyBucket] {
        let providers = self.providers
        if providers.isEmpty {
            return []
        }

        var providerRows: [String: [TokfenceStatsRow]] = [:]
        for provider in providers {
            providerRows[provider] = try runner.fetchStats(period: "24h", provider: provider, by: "hour")
        }

        let calendar = Calendar.current
        let now = Date()
        var hours: [Date] = []
        for i in stride(from: 23, through: 0, by: -1) {
            if let hour = calendar.date(byAdding: .hour, value: -i, to: now) {
                let components = calendar.dateComponents([.year, .month, .day, .hour], from: hour)
                if let normalized = calendar.date(from: components) {
                    hours.append(normalized)
                }
            }
        }

        var buckets: [TokfenceHourlyBucket] = hours.map { TokfenceHourlyBucket(hourStart: $0, counts: [:]) }
        let keyFormatter = DateFormatter()
        keyFormatter.dateFormat = "yyyy-MM-dd HH:00"
        keyFormatter.timeZone = TimeZone(secondsFromGMT: 0)

        for provider in providers {
            for row in providerRows[provider] ?? [] {
                guard let ts = keyFormatter.date(from: row.group) else {
                    continue
                }
                let localComponents = calendar.dateComponents([.year, .month, .day, .hour], from: ts)
                guard let localHour = calendar.date(from: localComponents) else {
                    continue
                }
                if let idx = buckets.firstIndex(where: { calendar.isDate($0.hourStart, equalTo: localHour, toGranularity: .hour) }) {
                    buckets[idx].counts[provider, default: 0] += row.requestCount
                }
            }
        }
        return buckets
    }

    private func setError(_ message: String) {
        lastError = message
        showErrorToast = true
        toastDismissTask?.cancel()
        let snapshot = message
        toastDismissTask = Task {
            try? await Task.sleep(for: .seconds(4))
            await MainActor.run {
                if self.lastError == snapshot {
                    self.showErrorToast = false
                }
            }
        }
    }

    private func clearError() {
        toastDismissTask?.cancel()
        toastDismissTask = nil
        lastError = ""
        showErrorToast = false
    }

    private func budgetAlertLevel(for budget: TokfenceBudget?) -> BudgetAlertLevel {
        guard let budget, budget.limitCents > 0 else {
            return .none
        }
        let progress = Double(budget.currentSpendCents) / Double(max(1, budget.limitCents))
        if progress >= 1 {
            return .exceeded
        }
        if progress >= budgetWarningThreshold {
            return .warning
        }
        return .none
    }

    private func dayKey(for date: Date = Date()) -> String {
        let formatter = DateFormatter()
        formatter.calendar = Calendar(identifier: .gregorian)
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = .current
        formatter.dateFormat = "yyyy-MM-dd"
        return formatter.string(from: date)
    }

    private func persistBudgetAlertState() {
        let defaults = UserDefaults.standard
        defaults.set(lastBudgetAlertLevel.rawValue, forKey: budgetAlertLevelDefaultsKey)
        defaults.set(lastBudgetAlertDay, forKey: budgetAlertDayDefaultsKey)
    }

    private func maybeNotifyBudgetThreshold(previous: TokfenceBudget?, current: TokfenceBudget?) {
        let todayKey = dayKey()
        if lastBudgetAlertDay != todayKey {
            lastBudgetAlertDay = todayKey
            lastBudgetAlertLevel = .none
            persistBudgetAlertState()
        }

        let previousLevel = budgetAlertLevel(for: previous)
        let baselineLevel: BudgetAlertLevel = previousLevel.rawValue > lastBudgetAlertLevel.rawValue ? previousLevel : lastBudgetAlertLevel
        let currentLevel = budgetAlertLevel(for: current)
        guard currentLevel.rawValue > baselineLevel.rawValue else {
            return
        }

        guard let current, current.limitCents > 0 else {
            return
        }

        let progress = Double(current.currentSpendCents) / Double(max(1, current.limitCents))
        let percent = Int((progress * 100).rounded())
        let title: String
        let body: String
        switch currentLevel {
        case .warning:
            title = "Tokfence budget warning"
            body = "Daily budget is at \(percent)% (\(TokfenceFormatting.usd(cents: current.currentSpendCents)) / \(TokfenceFormatting.usd(cents: current.limitCents)))."
        case .exceeded:
            title = "Tokfence budget exceeded"
            body = "Daily budget exceeded (\(TokfenceFormatting.usd(cents: current.currentSpendCents)) / \(TokfenceFormatting.usd(cents: current.limitCents))). Requests may be blocked."
        case .none:
            return
        }

        sendLocalNotification(title: title, body: body)
        lastBudgetAlertLevel = currentLevel
        lastBudgetAlertDay = todayKey
        persistBudgetAlertState()
    }

    private func sendLocalNotification(title: String, body: String) {
        let center = UNUserNotificationCenter.current()
        center.getNotificationSettings { [weak self] settings in
            guard let self else { return }
            switch settings.authorizationStatus {
            case .authorized, .provisional, .ephemeral:
                Task { @MainActor in
                    self.enqueueNotification(title: title, body: body)
                }
            case .notDetermined:
                center.requestAuthorization(options: [.alert, .sound]) { [weak self] granted, _ in
                    guard let self else { return }
                    guard granted else { return }
                    Task { @MainActor in
                        self.enqueueNotification(title: title, body: body)
                    }
                }
            case .denied:
                break
            @unknown default:
                break
            }
        }
    }

    private func enqueueNotification(title: String, body: String) {
        let content = UNMutableNotificationContent()
        content.title = title
        content.body = body
        content.sound = .default
        let request = UNNotificationRequest(
            identifier: "tokfence-budget-\(UUID().uuidString)",
            content: content,
            trigger: nil
        )
        UNUserNotificationCenter.current().add(request)
    }
}
