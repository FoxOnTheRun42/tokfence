import AppKit
import SwiftUI

@main
struct TokfenceDesktopApp: App {
    @StateObject private var viewModel = TokfenceAppViewModel()

    var body: some Scene {
        WindowGroup {
            ContentView(viewModel: viewModel)
                .frame(minWidth: 900, minHeight: 600)
                .task { viewModel.start() }
                .onDisappear { viewModel.stop() }
        }
        .commands {
            TokfenceAppCommands(viewModel: viewModel)
        }

        MenuBarExtra {
            TokfenceMenuBarView(viewModel: viewModel)
                .frame(width: 280)
        } label: {
            Image(systemName: viewModel.snapshot.running ? "lock.shield.fill" : "lock.slash")
        }
    }
}

private struct TokfenceAppCommands: Commands {
    @ObservedObject var viewModel: TokfenceAppViewModel

    var body: some Commands {
        CommandMenu("Tokfence") {
            Button("Dashboard") { viewModel.selectedSection = .dashboard }
                .keyboardShortcut("1", modifiers: .command)
            Button("Vault") { viewModel.selectedSection = .vault }
                .keyboardShortcut("2", modifiers: .command)
            Button("Logs") { viewModel.selectedSection = .logs }
                .keyboardShortcut("3", modifiers: .command)
            Button("Budget") { viewModel.selectedSection = .budget }
                .keyboardShortcut("4", modifiers: .command)
            Button("Providers") { viewModel.selectedSection = .providers }
                .keyboardShortcut("5", modifiers: .command)
            Divider()
            Button("Settings") { viewModel.selectedSection = .settings }
                .keyboardShortcut(",", modifiers: .command)
            Divider()
            Button("Refresh") { Task { await viewModel.refreshAll() } }
                .keyboardShortcut("r", modifiers: .command)
            Button(viewModel.snapshot.killSwitchActive ? "Deactivate Kill Switch" : "Activate Kill Switch") {
                Task {
                    if viewModel.snapshot.killSwitchActive {
                        await viewModel.killSwitchOff()
                    } else {
                        await viewModel.killSwitchOn()
                    }
                }
            }
            .keyboardShortcut("k", modifiers: [.command, .shift])
        }
    }
}

private struct TokfenceMenuBarView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            HStack {
                TokfenceStatusDot(
                    color: viewModel.snapshot.running ? TokfenceTheme.healthy : TokfenceTheme.danger,
                    label: viewModel.snapshot.running ? "Online" : "Offline"
                )
                Spacer()
                Text(TokfenceFormatting.usd(cents: viewModel.snapshot.todayCostCents))
                    .font(.system(size: 12, weight: .semibold, design: .monospaced))
            }

            Text("\(viewModel.snapshot.todayRequests) requests")
                .font(.system(size: 12, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)

            if let budget = viewModel.globalDailyBudget {
                Text("Budget: \(TokfenceFormatting.percentString(current: budget.currentSpendCents, limit: budget.limitCents))")
                    .font(.system(size: 11, weight: .regular))
                    .foregroundStyle(TokfenceTheme.textSecondary)
            }

            Divider()

            Button("Refresh") {
                Task { await viewModel.refreshAll() }
            }
            .buttonStyle(.plain)

            Button(viewModel.snapshot.running ? "Stop Daemon" : "Start Daemon") {
                Task {
                    if viewModel.snapshot.running {
                        await viewModel.stopDaemon()
                    } else {
                        await viewModel.startDaemon()
                    }
                }
            }
            .buttonStyle(.plain)

            Button(viewModel.snapshot.killSwitchActive ? "Kill Switch: ON" : "Kill Switch: OFF") {
                Task {
                    if viewModel.snapshot.killSwitchActive {
                        await viewModel.killSwitchOff()
                    } else {
                        await viewModel.killSwitchOn()
                    }
                }
            }
            .buttonStyle(.plain)

            Divider()

            Button("Open Tokfence") {
                NSApplication.shared.activate(ignoringOtherApps: true)
            }
            .buttonStyle(.plain)

            Button("Quit") {
                NSApplication.shared.terminate(nil)
            }
            .buttonStyle(.plain)
        }
        .padding(12)
        .onAppear {
            Task { await viewModel.refreshAll() }
        }
    }
}
