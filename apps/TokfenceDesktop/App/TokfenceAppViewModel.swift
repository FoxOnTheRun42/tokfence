import AppKit
import Foundation
import SwiftUI
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

@MainActor
final class TokfenceAppViewModel: ObservableObject {
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

    private var runner = TokfenceCommandRunner()
    private var refreshTask: Task<Void, Never>?
    private var tickCount = 0
    private var refreshTaskInProgress = false
    private var logsRefreshInProgress = false
    private var toastDismissTask: Task<Void, Never>?

    init() {
        binaryPath = runner.configuredBinaryPath
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

    func setBudget(provider: String, amountUSD: Double, period: String) async {
        await runAndRefresh { try runner.setBudget(provider: provider, amountUSD: amountUSD, period: period) }
    }

    func clearBudget(provider: String) async {
        await runAndRefresh { try runner.clearBudget(provider: provider) }
    }

    func addVaultKey(provider: String, key: String) async {
        await runAndRefresh { try runner.addVaultKey(provider: provider, key: key) }
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

    func openDataFolder() {
        let url = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".tokfence")
        NSWorkspace.shared.open(url)
    }

    private func runAndRefresh(_ operation: () throws -> Void) async {
        do {
            try operation()
            await refreshAll()
        } catch {
            setError(error.localizedDescription)
        }
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
}
