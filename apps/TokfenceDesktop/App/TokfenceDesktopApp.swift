import AppKit
import SwiftUI

@main
struct TokfenceDesktopApp: App {
    @StateObject private var viewModel = TokfenceAppViewModel()

    var body: some Scene {
        WindowGroup {
            ContentView(viewModel: viewModel)
                .frame(minWidth: TokfenceTheme.minWindowWidth, minHeight: TokfenceTheme.minWindowHeight)
                .task { viewModel.start() }
                .onDisappear { viewModel.stop() }
        }
        .defaultSize(width: TokfenceTheme.preferredWindowWidth, height: TokfenceTheme.preferredWindowHeight)
        .commands {
            TokfenceAppCommands(viewModel: viewModel)
        }

        MenuBarExtra {
            TokfenceMenuBarView(viewModel: viewModel)
                .frame(width: 280)
        } label: {
            TokfenceIcon(
                kind: viewModel.snapshot.killSwitchActive ? .menuAlert : (viewModel.snapshot.running ? .menuActive : .menuInactive),
                size: 16,
                primary: .black,
                accent: .black
            )
        }
    }
}

private struct TokfenceAppCommands: Commands {
    @ObservedObject var viewModel: TokfenceAppViewModel

    var body: some Commands {
        CommandMenu("Tokfence") {
            Button("Agents") { viewModel.selectedSection = .agents }
                .keyboardShortcut("1", modifiers: .command)
            Divider()
            Button("Overview") { viewModel.selectedSection = .overview }
                .keyboardShortcut("2", modifiers: .command)
            Button("Vault") { viewModel.selectedSection = .vault }
                .keyboardShortcut("3", modifiers: .command)
            Button("Activity") { viewModel.selectedSection = .activity }
                .keyboardShortcut("4", modifiers: .command)
            Button("Budget") { viewModel.selectedSection = .budget }
                .keyboardShortcut("5", modifiers: .command)
            Button("Providers") { viewModel.selectedSection = .providers }
                .keyboardShortcut("6", modifiers: .command)
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
                Text(viewModel.snapshot.running ? "\u{25CF} Online" : "\u{25CF} Offline")
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(viewModel.snapshot.running ? TokfenceTheme.healthy : TokfenceTheme.danger)
                Spacer()
                Text("\(TokfenceFormatting.usd(cents: viewModel.snapshot.todayCostCents)) today")
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

            Button(viewModel.snapshot.killSwitchActive ? "Kill switch: ON" : "Kill switch: OFF") {
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
