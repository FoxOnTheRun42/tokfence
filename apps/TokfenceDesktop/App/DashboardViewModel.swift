import Foundation
import SwiftUI

@MainActor
final class DashboardViewModel: ObservableObject {
    @Published var snapshot: TokfenceSnapshot = TokfenceSharedStore.loadSnapshot()
    @Published var isRefreshing = false
    @Published var lastError: String = ""
    @Published var binaryPath: String = ""

    private var runner = TokfenceCommandRunner()
    private var refreshTask: Task<Void, Never>?

    init() {
        binaryPath = runner.configuredBinaryPath
    }

    func start() {
        refreshTask?.cancel()
        refreshTask = Task { [weak self] in
            guard let self else { return }
            await self.refresh()
            while !Task.isCancelled {
                try? await Task.sleep(for: .seconds(20))
                await self.refresh()
            }
        }
    }

    func stop() {
        refreshTask?.cancel()
        refreshTask = nil
    }

    func saveBinaryPath() {
        runner.configuredBinaryPath = binaryPath
        Task { await refresh() }
    }

    func refresh() async {
        isRefreshing = true
        defer { isRefreshing = false }
        do {
            let latest = try runner.fetchSnapshot()
            snapshot = latest
            TokfenceSharedStore.saveSnapshot(latest)
            lastError = ""
        } catch {
            lastError = error.localizedDescription
        }
    }

    func actionStart() async { await runAction(["start", "-d"]) }
    func actionStop() async { await runAction(["stop"]) }
    func actionKill() async { await runAction(["kill"]) }
    func actionUnkill() async { await runAction(["unkill"]) }

    func openLogs() {
        try? runner.runAction(arguments: ["log", "-f"])
    }

    func openDataFolder() {
        let url = URL(fileURLWithPath: NSHomeDirectory()).appendingPathComponent(".tokfence")
        NSWorkspace.shared.open(url)
    }

    private func runAction(_ args: [String]) async {
        do {
            try runner.runAction(arguments: args)
            await refresh()
        } catch {
            lastError = error.localizedDescription
        }
    }
}
