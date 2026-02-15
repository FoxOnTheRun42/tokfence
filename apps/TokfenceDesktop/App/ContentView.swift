import SwiftUI

struct ContentView: View {
    @ObservedObject var viewModel: DashboardViewModel

    var body: some View {
        ZStack {
            LinearGradient(
                colors: [Color(red: 0.07, green: 0.10, blue: 0.18), Color(red: 0.12, green: 0.08, blue: 0.16)],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .ignoresSafeArea()

            ScrollView {
                VStack(alignment: .leading, spacing: 18) {
                    header
                    statusRow
                    metricsRow
                    budgetSection
                    accessSection
                    actionsSection

                    if !viewModel.lastError.isEmpty {
                        Text(viewModel.lastError)
                            .font(.system(size: 13, weight: .medium, design: .rounded))
                            .padding(12)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .foregroundStyle(Color(red: 0.95, green: 0.35, blue: 0.35))
                            .background(Color.white.opacity(0.06), in: RoundedRectangle(cornerRadius: 12))
                    }
                }
                .padding(24)
            }
        }
    }

    private var header: some View {
        HStack(alignment: .top) {
            VStack(alignment: .leading, spacing: 6) {
                Text("Tokfence Desktop")
                    .font(.system(size: 36, weight: .bold, design: .rounded))
                    .foregroundStyle(.white)
                Text("SwiftUI dashboard + WidgetKit feed")
                    .font(.system(size: 14, weight: .medium, design: .rounded))
                    .foregroundStyle(.white.opacity(0.7))
            }
            Spacer()
            HStack(spacing: 12) {
                TextField("Tokfence binary path", text: $viewModel.binaryPath)
                    .textFieldStyle(.roundedBorder)
                    .frame(width: 360)
                Button("Save") {
                    viewModel.saveBinaryPath()
                }
                .buttonStyle(.borderedProminent)
                Button {
                    Task { await viewModel.refresh() }
                } label: {
                    if viewModel.isRefreshing {
                        ProgressView()
                            .progressViewStyle(.circular)
                    } else {
                        Text("Refresh")
                    }
                }
                .buttonStyle(.bordered)
            }
        }
    }

    private var statusRow: some View {
        HStack(spacing: 14) {
            statusBadge(title: "Daemon", value: viewModel.snapshot.running ? "Online" : "Offline", color: viewModel.snapshot.running ? .green : .red)
            statusBadge(title: "Address", value: viewModel.snapshot.addr ?? "n/a", color: .blue)
            statusBadge(title: "Last Request", value: viewModel.snapshot.lastRequestAt ?? "none", color: .orange)
            statusBadge(title: "Vault Providers", value: "\(viewModel.snapshot.vaultProviders.count)", color: .purple)
        }
    }

    private var metricsRow: some View {
        HStack(spacing: 14) {
            metricCard(title: "Today's Cost", value: TokfenceFormatting.usd(cents: viewModel.snapshot.todayCostCents), icon: "dollarsign.circle.fill", tint: Color(red: 0.10, green: 0.65, blue: 0.42))
            metricCard(title: "Requests", value: "\(viewModel.snapshot.todayRequests)", icon: "waveform.path.ecg.rectangle", tint: Color(red: 0.18, green: 0.58, blue: 0.95))
            metricCard(title: "Input Tokens", value: TokfenceFormatting.tokens(viewModel.snapshot.todayInputTokens), icon: "arrow.down.right.circle.fill", tint: Color(red: 0.95, green: 0.63, blue: 0.13))
            metricCard(title: "Output Tokens", value: TokfenceFormatting.tokens(viewModel.snapshot.todayOutputTokens), icon: "arrow.up.right.circle.fill", tint: Color(red: 0.87, green: 0.32, blue: 0.31))
        }
    }

    private var budgetSection: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Budgets")
                .font(.system(size: 18, weight: .semibold, design: .rounded))
                .foregroundStyle(.white)

            if viewModel.snapshot.budgets.isEmpty {
                EmptyStateCard(text: "No budgets configured yet.")
            } else {
                ForEach(viewModel.snapshot.budgets) { budget in
                    let progress = TokfenceFormatting.percent(current: budget.currentSpendCents, limit: budget.limitCents)
                    VStack(alignment: .leading, spacing: 8) {
                        HStack {
                            Text(budget.provider)
                                .font(.system(size: 14, weight: .bold, design: .rounded))
                                .foregroundStyle(.white)
                            Spacer()
                            Text("\(TokfenceFormatting.usd(cents: budget.currentSpendCents)) / \(TokfenceFormatting.usd(cents: budget.limitCents))")
                                .font(.system(size: 13, weight: .semibold, design: .rounded))
                                .foregroundStyle(.white.opacity(0.85))
                        }
                        ProgressView(value: progress)
                            .tint(progress < 0.8 ? .green : (progress < 1.0 ? .orange : .red))
                        Text("Period: \(budget.period)")
                            .font(.system(size: 12, weight: .medium, design: .rounded))
                            .foregroundStyle(.white.opacity(0.7))
                    }
                    .padding(12)
                    .background(Color.white.opacity(0.06), in: RoundedRectangle(cornerRadius: 12))
                }
            }
        }
    }

    private var accessSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Access Control")
                .font(.system(size: 18, weight: .semibold, design: .rounded))
                .foregroundStyle(.white)

            if viewModel.snapshot.revokedProviders.isEmpty {
                EmptyStateCard(text: "No providers are revoked.")
            } else {
                Text("Revoked: \(viewModel.snapshot.revokedProviders.joined(separator: ", "))")
                    .font(.system(size: 13, weight: .semibold, design: .rounded))
                    .foregroundStyle(Color(red: 0.95, green: 0.45, blue: 0.45))
                    .padding(12)
                    .frame(maxWidth: .infinity, alignment: .leading)
                    .background(Color.white.opacity(0.06), in: RoundedRectangle(cornerRadius: 12))
            }

            if !viewModel.snapshot.warnings.isEmpty {
                VStack(alignment: .leading, spacing: 6) {
                    ForEach(viewModel.snapshot.warnings, id: \.self) { warning in
                        Text("• \(warning)")
                            .font(.system(size: 12, weight: .medium, design: .rounded))
                            .foregroundStyle(Color(red: 0.98, green: 0.79, blue: 0.42))
                    }
                }
                .padding(12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(Color.white.opacity(0.06), in: RoundedRectangle(cornerRadius: 12))
            }
        }
    }

    private var actionsSection: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Quick Actions")
                .font(.system(size: 18, weight: .semibold, design: .rounded))
                .foregroundStyle(.white)

            HStack(spacing: 10) {
                actionButton("Start", systemImage: "play.fill", tint: .green) { Task { await viewModel.actionStart() } }
                actionButton("Stop", systemImage: "stop.fill", tint: .orange) { Task { await viewModel.actionStop() } }
                actionButton("Kill", systemImage: "bolt.fill", tint: .red) { Task { await viewModel.actionKill() } }
                actionButton("Restore", systemImage: "shield.fill", tint: .blue) { Task { await viewModel.actionUnkill() } }
                actionButton("Open Folder", systemImage: "folder.fill", tint: .purple) { viewModel.openDataFolder() }
            }
        }
    }

    private func statusBadge(title: String, value: String, color: Color) -> some View {
        VStack(alignment: .leading, spacing: 4) {
            Text(title)
                .font(.system(size: 11, weight: .semibold, design: .rounded))
                .foregroundStyle(.white.opacity(0.72))
            Text(value)
                .font(.system(size: 14, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
                .lineLimit(1)
        }
        .padding(.vertical, 10)
        .padding(.horizontal, 12)
        .background(color.opacity(0.22), in: RoundedRectangle(cornerRadius: 12))
    }

    private func metricCard(title: String, value: String, icon: String, tint: Color) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            Image(systemName: icon)
                .font(.system(size: 20, weight: .bold))
                .foregroundStyle(tint)
            Text(title)
                .font(.system(size: 12, weight: .semibold, design: .rounded))
                .foregroundStyle(.white.opacity(0.75))
            Text(value)
                .font(.system(size: 24, weight: .bold, design: .rounded))
                .foregroundStyle(.white)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(14)
        .background(Color.white.opacity(0.06), in: RoundedRectangle(cornerRadius: 14))
    }

    private func actionButton(_ title: String, systemImage: String, tint: Color, action: @escaping () -> Void) -> some View {
        Button(action: action) {
            HStack(spacing: 6) {
                Image(systemName: systemImage)
                Text(title)
            }
            .font(.system(size: 13, weight: .bold, design: .rounded))
            .padding(.vertical, 9)
            .padding(.horizontal, 12)
        }
        .buttonStyle(.plain)
        .foregroundStyle(.white)
        .background(tint.opacity(0.7), in: Capsule())
    }
}

private struct EmptyStateCard: View {
    let text: String

    var body: some View {
        Text(text)
            .font(.system(size: 13, weight: .medium, design: .rounded))
            .foregroundStyle(.white.opacity(0.8))
            .padding(12)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(Color.white.opacity(0.06), in: RoundedRectangle(cornerRadius: 12))
    }
}
