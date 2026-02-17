import AppKit
import Charts
import SwiftUI

struct ContentView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    var body: some View {
        HStack(spacing: 0) {
            sidebar
                .frame(width: TokfenceTheme.sidebarWidth)
                .background(TokfenceTheme.bgSecondary)
                .overlay(
                    Divider().offset(x: TokfenceTheme.sidebarWidth),
                    alignment: .trailing
                )

            mainContent
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .background(TokfenceTheme.bgPrimary)
                .animation(TokfenceTheme.uiAnimation, value: viewModel.selectedSection)
                .overlay(alignment: .top) {
                    if !viewModel.lastError.isEmpty && viewModel.showErrorToast {
                        TokfenceToast(
                            message: viewModel.lastError,
                            tone: TokfenceTheme.danger,
                            action: nil,
                            onClose: {
                                withAnimation(TokfenceTheme.uiAnimation) {
                                    viewModel.dismissErrorToast()
                                }
                            }
                        )
                        .padding(.top, 8)
                        .padding(.horizontal, 12)
                        .frame(maxWidth: 320)
                        .transition(.move(edge: .top).combined(with: .opacity))
                        .zIndex(1)
                    }
                }
        }
        .animation(TokfenceTheme.uiAnimation, value: viewModel.selectedSection)
        .alert("Daemon identity mismatch", isPresented: $viewModel.showDaemonIdentityMismatchDialog) {
            Button("Retry") {
                Task { await viewModel.refreshAll() }
            }
            Button("Clear stale pid file", role: .destructive) {
                Task { await viewModel.clearDaemonPIDFile() }
            }
            Button("Dismiss", role: .cancel) {
                viewModel.dismissDaemonIdentityMismatch()
            }
        } message: {
            Text(viewModel.daemonIdentityMismatchError)
        }
        .background(TokfenceTheme.bgPrimary)
    }

    private var sidebar: some View {
        VStack(alignment: .leading, spacing: 6) {
            // MARK: - Brand header
            HStack(spacing: TokfenceTheme.spaceSm) {
                TokfenceLogoMark()
                Text("Tokfence")
                    .font(.system(size: 15, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.textPrimary)
                Spacer(minLength: 0)
                DaemonStatusMenu(viewModel: viewModel)
            }
            .frame(height: 32)
            .padding(.bottom, 8)

            // MARK: - Primary zone: Agents
            TokfenceSidebarGroupLabel(title: "My Agents")

            ForEach(TokfenceSection.primary, id: \.self) { section in
                TokfenceNavItem(
                    isSelected: viewModel.selectedSection == section,
                    title: section.title,
                    icon: section.symbol,
                    badgeText: section == .agents && viewModel.runningAgentsCount > 0 ? "\(viewModel.runningAgentsCount)" : nil,
                    style: .primary
                ) {
                    withAnimation(TokfenceTheme.uiSpring) {
                        viewModel.selectedSection = section
                    }
                }
            }

            // MARK: - Running agent sub-items
            if viewModel.runningAgentsCount > 0 && viewModel.selectedSection != .agents {
                HStack(spacing: 6) {
                    Text("↳")
                        .font(.system(size: 11, weight: .medium, design: .monospaced))
                        .foregroundStyle(TokfenceTheme.textTertiary)
                    Text("OpenClaw")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                    Circle()
                        .fill(TokfenceTheme.healthy)
                        .frame(width: 6, height: 6)
                }
                .padding(.leading, 40)
                .onTapGesture {
                    withAnimation(TokfenceTheme.uiSpring) {
                        viewModel.selectedSection = .agents
                    }
                }
            }

            Divider()
                .padding(.vertical, 4)

            // MARK: - Secondary zone: Proxy
            TokfenceSidebarGroupLabel(title: "Proxy")

            ForEach(TokfenceSection.proxy, id: \.self) { section in
                TokfenceNavItem(
                    isSelected: viewModel.selectedSection == section,
                    title: section.title,
                    icon: section.symbol,
                    style: .secondary
                ) {
                    withAnimation(TokfenceTheme.uiSpring) {
                        viewModel.selectedSection = section
                    }
                }
            }

            Spacer(minLength: TokfenceTheme.spaceMd)

            // MARK: - Utility zone: Settings
            ForEach(TokfenceSection.utility, id: \.self) { section in
                TokfenceNavItem(
                    isSelected: viewModel.selectedSection == section,
                    title: section.title,
                    icon: section.symbol,
                    style: .secondary
                ) {
                    withAnimation(TokfenceTheme.uiSpring) {
                        viewModel.selectedSection = section
                    }
                }
            }

            // MARK: - Kill switch badge
            if viewModel.snapshot.killSwitchActive {
                TokfenceStatusBadge(
                    label: "KILLED",
                    icon: "bolt.slash.fill",
                    tint: TokfenceTheme.danger
                )
            }

            // MARK: - Budget footer
            let budget = viewModel.globalDailyBudget
            VStack(alignment: .leading, spacing: 4) {
                Text("Today \(TokfenceFormatting.usd(cents: budget?.currentSpendCents ?? viewModel.snapshot.todayCostCents)) / \(TokfenceFormatting.usd(cents: budget?.limitCents ?? max(1, viewModel.snapshot.todayCostCents + 1)))")
                    .font(.system(size: TokfenceTheme.fontCaption, weight: .medium))
                    .foregroundStyle(TokfenceTheme.textPrimary)
                TokfenceBudgetProgressBar(
                    current: budget?.currentSpendCents ?? viewModel.snapshot.todayCostCents,
                    limit: max(budget?.limitCents ?? 1, 1)
                )
            }
            .padding(12)
            .background(TokfenceTheme.bgSecondary, in: RoundedRectangle(cornerRadius: TokfenceTheme.cardCorner, style: .continuous))
        }
        .padding(.top, 24)
        .padding(.horizontal, 16)
    }

    @ViewBuilder
    private var mainContent: some View {
        ZStack {
            switch viewModel.selectedSection {
            case .agents:
                AgentsSectionView(viewModel: viewModel)
            case .overview:
                OverviewSectionView(viewModel: viewModel)
            case .vault:
                VaultSectionView(viewModel: viewModel)
            case .activity:
                ActivitySectionView(viewModel: viewModel)
            case .budget:
                BudgetSectionView(viewModel: viewModel)
            case .providers:
                ProvidersSectionView(viewModel: viewModel)
            case .settings:
                SettingsSectionView(viewModel: viewModel)
            }
        }
        .id(viewModel.selectedSection)
        .padding(24)
        .transition(.opacity)
    }
}

private struct TokfenceLogoMark: View {
    var body: some View {
        RoundedRectangle(cornerRadius: 6, style: .continuous)
            .fill(TokfenceTheme.accentPrimary)
            .frame(width: 24, height: 24)
            .overlay {
                Image(systemName: "lock.fill")
                    .font(.system(size: 12, weight: .bold))
                    .foregroundStyle(.white)
            }
    }
}

private struct DaemonStatusMenu: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    private var statusColor: Color {
        if viewModel.snapshot.killSwitchActive {
            return TokfenceTheme.danger
        }
        return viewModel.snapshot.running ? TokfenceTheme.healthy : TokfenceTheme.textTertiary
    }

    var body: some View {
        Menu {
            Button("Refresh status") { Task { await viewModel.refreshAll() } }
            Button(viewModel.snapshot.running ? "Stop daemon" : "Start daemon") {
                Task {
                    if viewModel.snapshot.running {
                        await viewModel.stopDaemon()
                    } else {
                        await viewModel.startDaemon()
                    }
                }
            }
            Divider()
            Button("Restart daemon") {
                Task {
                    await viewModel.stopDaemon()
                    await viewModel.startDaemon()
                }
            }
            Divider()
            Button("Open data folder") {
                viewModel.openDataFolder()
            }
        } label: {
            Circle()
                .fill(statusColor)
                .overlay(
                    Circle()
                        .stroke(TokfenceTheme.bgPrimary, lineWidth: 1.5)
                )
                .frame(width: 10, height: 10)
        }
        .menuStyle(.borderlessButton)
        .help("Daemon controls")
    }
}

private struct RequestListRow: View {
    let record: TokfenceLogRecord
    let index: Int
    let isSelected: Bool
    let isCompact: Bool

    var body: some View {
        HStack(spacing: 8) {
            Text(TokfenceFormatting.timeOfDay(record.timestamp))
                .font(.system(size: 11, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textSecondary)
                .frame(width: 70, alignment: .leading)

            TokfenceProviderBadge(provider: record.provider, active: true)
                .frame(width: 80, alignment: .leading)

            Text(record.model.isEmpty ? "(unknown)" : record.model)
                .font(.system(size: 11, weight: .regular, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
                .lineLimit(1)
                .frame(width: isCompact ? 140 : 180, alignment: .leading)

            Text("\(TokfenceFormatting.tokens(record.inputTokens)) → \(TokfenceFormatting.tokens(record.outputTokens))")
                .font(.system(size: 11, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
                .frame(width: 94, alignment: .trailing)

            Text(TokfenceFormatting.usd(cents: record.estimatedCostCents))
                .font(.system(size: 11, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
                .frame(width: 64, alignment: .trailing)

            Text(TokfenceFormatting.latency(ms: record.latencyMS))
                .font(.system(size: 11, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
                .frame(width: 64, alignment: .trailing)

            Circle()
                .fill(TokfenceTheme.statusColor(for: record.statusCode))
                .frame(width: 8, height: 8)
                .frame(width: isCompact ? 50 : 56, alignment: .leading)
                .accessibilityLabel("status \(record.statusCode)")

            if !isCompact {
                HStack(spacing: 4) {
                    if record.isStreaming {
                        Image(systemName: "waveform")
                    }
                    Text(record.isStreaming ? "stream" : "batch")
                        .font(.system(size: 10, weight: .medium))
                }
                .foregroundStyle(TokfenceTheme.textSecondary)
                .frame(width: 70, alignment: .leading)
            }
        }
        .padding(.horizontal, 12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .frame(height: 32)
        .background(rowFill, in: RoundedRectangle(cornerRadius: TokfenceTheme.cardCorner, style: .continuous))
        .contentShape(Rectangle())
    }

    private var rowFill: Color {
        if isSelected {
            return TokfenceTheme.accentMuted
        }
        if index % 2 == 0 {
            return TokfenceTheme.bgSecondary
        }
        return TokfenceTheme.bgTertiary.opacity(0.4)
    }
}

private struct RequestListPanel: View {
    let records: [TokfenceLogRecord]
    @Binding var selectedRequestID: String?
    var compact: Bool = false

    var body: some View {
        let displayRecords = compact ? Array(records.prefix(3)) : records
        return VStack(spacing: 8) {
            HStack(spacing: 8) {
                Text("Time")
                    .frame(width: 70, alignment: .leading)
                Text("Provider")
                    .frame(width: 80, alignment: .leading)
                Text("Model")
                    .frame(width: compact ? 140 : 180, alignment: .leading)
                Text("Tokens")
                    .frame(width: 94, alignment: .trailing)
                Text("Cost")
                    .frame(width: 64, alignment: .trailing)
                Text("Latency")
                    .frame(width: 64, alignment: .trailing)
                Text("Status")
                    .frame(width: compact ? 50 : 56, alignment: .leading)
                if !compact {
                    Text("Type")
                        .frame(width: 70, alignment: .leading)
                }

                Spacer(minLength: 0)
            }
            .font(.system(size: 10, weight: .medium))
            .foregroundStyle(TokfenceTheme.textSecondary)
            .padding(.horizontal, 12)
            .padding(.vertical, 6)
            .background(TokfenceTheme.bgTertiary, in: RoundedRectangle(cornerRadius: TokfenceTheme.badgeCorner, style: .continuous))

            Group {
                if compact {
                    VStack(spacing: 8) {
                        ForEach(Array(displayRecords.enumerated()), id: \.element.id) { index, record in
                            Button {
                                withAnimation(TokfenceTheme.uiSpring) {
                                    selectedRequestID = record.id
                                }
                            } label: {
                                RequestListRow(
                                    record: record,
                                    index: index,
                                    isSelected: selectedRequestID == record.id,
                                    isCompact: compact
                                )
                            }
                            .buttonStyle(.plain)
                        }
                    }
                } else {
                    ScrollView {
                        LazyVStack(spacing: 8) {
                            ForEach(Array(displayRecords.enumerated()), id: \.element.id) { index, record in
                                Button {
                                    withAnimation(TokfenceTheme.uiSpring) {
                                        selectedRequestID = record.id
                                    }
                                } label: {
                                    RequestListRow(
                                        record: record,
                                        index: index,
                                        isSelected: selectedRequestID == record.id,
                                        isCompact: compact
                                    )
                                }
                                .buttonStyle(.plain)
                            }
                        }
                    }
                    .frame(maxHeight: .infinity)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
    }
}
private struct OverviewSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel
    @State private var selectedRequestID: String?

    private struct ChartPoint: Identifiable {
        let hourStart: Date
        let provider: String
        let count: Int

        var id: String {
            let ts = Int(hourStart.timeIntervalSince1970)
            return "\(provider)-\(ts)"
        }
    }

    private var chartPoints: [ChartPoint] {
        var points: [ChartPoint] = []
        for bucket in viewModel.hourlyBuckets {
            for provider in viewModel.providers {
                let count = bucket.counts[provider, default: 0]
                if count > 0 {
                    points.append(ChartPoint(hourStart: bucket.hourStart, provider: provider, count: count))
                }
            }
        }
        return points
    }

    private var chartDomain: ClosedRange<Date>? {
        guard
            let first = viewModel.hourlyBuckets.first?.hourStart,
            let last = viewModel.hourlyBuckets.last?.hourStart
        else {
            return nil
        }
        return first ... last.addingTimeInterval(3600)
    }

    private var medianLatencyMS: Int {
        let values = viewModel.logs.map(\.latencyMS).sorted()
        guard !values.isEmpty else { return 0 }
        return values[values.count / 2]
    }

    private var averageHourlyRequests: Double {
        guard !viewModel.hourlyBuckets.isEmpty else { return 0 }
        let total = viewModel.hourlyBuckets.reduce(0) { $0 + $1.totalCount }
        return Double(total) / Double(viewModel.hourlyBuckets.count)
    }

    private var requestsTrendText: String {
        let calendar = Calendar.current
        let startToday = calendar.startOfDay(for: Date())
        guard let startYesterday = calendar.date(byAdding: .day, value: -1, to: startToday) else {
            return ""
        }
        let yesterday = viewModel.logs.filter {
            $0.timestamp >= startYesterday && $0.timestamp < startToday
        }.count
        let today = viewModel.snapshot.todayRequests
        if yesterday <= 0 {
            return "n/a vs yesterday"
        }
        let pct = ((Double(today - yesterday) / Double(yesterday)) * 100.0).rounded()
        let prefix = pct >= 0 ? "+" : ""
        return "\(prefix)\(Int(pct))% vs yesterday"
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                dashboardHeader
                dashboardMetrics
                dashboardActivityCard
                dashboardRecentCard
            }
        }
        .sheet(item: Binding(
            get: { selectedRequestID.flatMap { id in viewModel.logs.first(where: { $0.id == id }) } },
            set: { item in selectedRequestID = item?.id }
        )) { record in
            LogDetailPanel(record: record)
                .frame(minWidth: 640, minHeight: 430)
                .padding(16)
        }
    }

    private var dashboardHeader: some View {
        TokfenceSectionHeader(
            title: "Overview",
            subtitle: "What happened today",
            trailing: AnyView(
                Button {
                    Task { await viewModel.refreshAll() }
                } label: {
                    if viewModel.isRefreshing {
                        ProgressView()
                    } else {
                        Label("Refresh", systemImage: "arrow.clockwise")
                    }
                }
                .buttonStyle(.bordered)
            )
        )
    }

    private var dashboardMetrics: some View {
        HStack(spacing: 12) {
            TokfenceMetricCard(
                icon: "arrow.up.arrow.down",
                value: "\(viewModel.snapshot.todayRequests)",
                title: "Requests",
                subtitle: "today",
                trend: requestsTrendText
            )
            TokfenceMetricCard(
                icon: "textformat.123",
                value: TokfenceFormatting.tokens(viewModel.snapshot.todayInputTokens + viewModel.snapshot.todayOutputTokens),
                title: "Tokens",
                subtitle: "in + out"
            )
            TokfenceMetricCard(
                icon: "dollarsign.circle",
                value: TokfenceFormatting.usd(cents: viewModel.snapshot.todayCostCents),
                title: "Cost",
                subtitle: "estimated today"
            )
            TokfenceMetricCard(
                icon: "clock",
                value: TokfenceFormatting.latency(ms: medianLatencyMS),
                title: "Latency",
                subtitle: "median p50"
            )
        }
        .frame(height: 160)
    }

    private var dashboardActivityCard: some View {
        TokfenceCard {
            Text("Activity (24h)")
                .font(.system(size: 16, weight: .medium))
                .foregroundStyle(TokfenceTheme.textPrimary)

            if chartPoints.isEmpty {
                TokfenceEmptyState(
                    symbol: "chart.bar",
                    title: "No activity yet",
                    message: "Start an AI agent and point it to localhost:9471.",
                    actionTitle: nil,
                    action: nil
                )
            } else {
                dashboardActivityChart
            }
        }
        .frame(height: 220)
    }

    private var dashboardActivityChart: some View {
        Chart {
            ForEach(chartPoints) { point in
                BarMark(
                    x: .value("Hour", point.hourStart, unit: .hour),
                    y: .value("Requests", point.count),
                    width: .fixed(12)
                )
                .foregroundStyle(by: .value("Provider", TokfenceFormatting.providerLabel(point.provider)))
            }
            if averageHourlyRequests > 0 {
                RuleMark(y: .value("Average", averageHourlyRequests))
                    .lineStyle(StrokeStyle(lineWidth: 1, dash: [3, 3]))
                    .foregroundStyle(TokfenceTheme.textTertiary)
            }
        }
        .frame(height: 180)
        .chartXScale(domain: chartDomain ?? Date()...Date().addingTimeInterval(24 * 3600))
        .chartXAxis {
            AxisMarks(values: .stride(by: .hour, count: 2)) { value in
                AxisGridLine()
                AxisTick()
                if value.as(Date.self) != nil {
                    AxisValueLabel(format: .dateTime.hour(.twoDigits(amPM: .omitted)))
                }
            }
        }
        .chartLegend(position: .bottom, spacing: 10)
    }

    private var dashboardRecentCard: some View {
        TokfenceCard {
            Text("Recent Requests")
                .font(.system(size: 16, weight: .medium))
                .foregroundStyle(TokfenceTheme.textPrimary)

            if viewModel.logs.isEmpty {
                TokfenceEmptyState(
                    symbol: "list.bullet.rectangle",
                    title: "No requests recorded yet",
                    message: "Requests will appear here after your first proxied call.",
                    actionTitle: nil,
                    action: nil
                )
            } else {
                RequestListPanel(records: Array(viewModel.logs.prefix(3)), selectedRequestID: $selectedRequestID, compact: true)
            }
        }
        .frame(height: 220)
    }

}

private struct VaultSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    @State private var editor: VaultKeyEditorState?
    @State private var providerToRemove: String?
    @State private var showSetupWizard = false

    private var configuredProviders: [String] {
        Array(Set(viewModel.snapshot.vaultProviders.map { $0.lowercased() })).sorted()
    }

    private func maskedKeyPreview(provider: String) -> String {
        if provider == "anthropic" {
            return "Key: sk-ant-************"
        }
        return "Key: sk-************"
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                TokfenceSectionHeader(
                    title: "Vault",
                    subtitle: "Manage API keys securely",
                    trailing: AnyView(
                        HStack(spacing: 8) {
                            Button {
                                showSetupWizard = true
                            } label: {
                                Label("Setup Wizard", systemImage: "wand.and.stars")
                            }
                            .buttonStyle(.bordered)

                            Button {
                                editor = VaultKeyEditorState(provider: "", mode: .add, upstream: "")
                            } label: {
                                Label("Add Key", systemImage: "plus")
                            }
                            .buttonStyle(.borderedProminent)
                            .tint(TokfenceTheme.accentPrimary)
                        }
                    )
                )

                if configuredProviders.isEmpty {
                    TokfenceEmptyState(
                        symbol: "key.fill",
                        title: "No API keys stored",
                        message: "Add your first provider key to get started.",
                        actionTitle: "Run Setup Wizard",
                        action: {
                            showSetupWizard = true
                        }
                    )
                }

                ForEach(configuredProviders, id: \.self) { provider in
                    let isRevoked = viewModel.snapshot.revokedProviders.contains(provider)
                    let lastUsed = viewModel.logs.first(where: { $0.provider == provider })?.timestamp
                    let upstream = viewModel.snapshot.providerUpstreams[provider] ?? ""

                    TokfenceCard {
                        HStack(alignment: .center) {
                            Circle()
                                .fill(isRevoked ? TokfenceTheme.warning : TokfenceTheme.healthy)
                                .frame(width: 10, height: 10)
                            Text(TokfenceFormatting.providerLabel(provider))
                                .font(.system(size: 16, weight: .semibold))
                                .foregroundStyle(TokfenceTheme.textPrimary)
                            Spacer()
                            Button("Rotate") {
                                editor = VaultKeyEditorState(provider: provider, mode: .rotate, upstream: upstream)
                            }
                            .buttonStyle(.bordered)

                            Button("Remove", role: .destructive) {
                                providerToRemove = provider
                            }
                            .buttonStyle(.bordered)
                        }

                        Text(maskedKeyPreview(provider: provider))
                            .font(.system(size: 12, weight: .medium, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textPrimary)

                        if !upstream.isEmpty {
                            Text(upstream)
                                .font(.system(size: 11, weight: .regular, design: .monospaced))
                                .foregroundStyle(TokfenceTheme.textSecondary)
                                .lineLimit(1)
                        }

                        Text(
                            "Last used: " + (lastUsed.map { TokfenceFormatting.relative($0) } ?? "never")
                        )
                            .font(.system(size: 11, weight: .regular))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                    }
                }

                TokfenceCard {
                    Text("Vault backend: macOS Keychain (default) · \(configuredProviders.count) keys stored")
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(TokfenceTheme.textPrimary)
                    HStack(spacing: 8) {
                        Button("Open Data Folder") {
                            viewModel.openDataFolder()
                        }
                        .buttonStyle(.bordered)

                        Button("Refresh") {
                            Task { await viewModel.refreshAll() }
                        }
                        .buttonStyle(.bordered)
                    }
                }
            }
        }
        .sheet(isPresented: $showSetupWizard) {
            SetupWizardSheet(viewModel: viewModel, knownUpstreams: viewModel.snapshot.providerUpstreams)
        }
        .sheet(item: $editor) { state in
            VaultKeyEditorSheet(state: state, knownUpstreams: viewModel.snapshot.providerUpstreams) { submission in
                Task {
                    if state.mode == .add {
                        await viewModel.addVaultKey(
                            provider: submission.provider,
                            key: submission.key,
                            endpoint: submission.endpoint,
                            preferredModel: submission.model
                        )
                    } else {
                        await viewModel.rotateVaultKey(provider: state.provider, key: submission.key)
                    }
                    editor = nil
                }
            }
        }
        .confirmationDialog(
            "Remove API key?",
            isPresented: Binding(
                get: { providerToRemove != nil },
                set: { if !$0 { providerToRemove = nil } }
            ),
            titleVisibility: .visible
        ) {
            Button("Remove", role: .destructive) {
                guard let providerToRemove else { return }
                Task { await viewModel.removeVaultKey(provider: providerToRemove) }
                self.providerToRemove = nil
            }
            Button("Cancel", role: .cancel) {
                providerToRemove = nil
            }
        } message: {
            Text("Agents using this provider will fail immediately.")
        }
    }
}

private struct ActivitySectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel
    @State private var selectedRequestID: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            TokfenceSectionHeader(
                title: "Activity",
                subtitle: "Live request stream"
            , trailing: AnyView(
                HStack(spacing: 8) {
                    Toggle(isOn: $viewModel.liveTailEnabled) {
                        Label("Live", systemImage: "waveform")
                            .font(.system(size: 12, weight: .medium))
                    }
                    .toggleStyle(.switch)
                    .labelsHidden()

                    HStack(spacing: 6) {
                        Circle()
                            .fill(viewModel.liveTailEnabled ? TokfenceTheme.info : TokfenceTheme.textTertiary)
                            .frame(width: 7, height: 7)
                        Text(viewModel.liveTailEnabled ? "Live" : "Paused")
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                    }

                    Button {
                        Task { await viewModel.refreshLogsOnly() }
                    } label: {
                        Label("Refresh", systemImage: "arrow.clockwise")
                    }
                    .buttonStyle(.bordered)
                }
            ))

            filterBar

            HSplitView {
                TokfenceCard {
                    if viewModel.filteredLogs.isEmpty {
                        TokfenceEmptyState(
                            symbol: "tray",
                            title: "No matching logs",
                            message: "Adjust filters or generate new traffic through the proxy.",
                            actionTitle: nil,
                            action: nil
                        )
                    } else {
                        RequestListPanel(records: viewModel.filteredLogs, selectedRequestID: $selectedRequestID)
                    }
                }
                .frame(maxHeight: .infinity)

                if let selectedRequestID, let record = viewModel.logs.first(where: { $0.id == selectedRequestID }) {
                    TokfenceCard {
                        LogDetailPanel(record: record)
                    }
                    .frame(width: 320)
                }
            }
            .frame(height: 500)
        }
        .onChange(of: viewModel.logTimeRange) { _, _ in
            Task {
                await viewModel.refreshLogsOnly()
            }
        }
        .onChange(of: viewModel.logProviderFilter) { _, _ in
            Task {
                await viewModel.refreshLogsOnly()
            }
        }
        .onChange(of: viewModel.logStatusFilter) { _, _ in
            Task {
                await viewModel.refreshLogsOnly()
            }
        }
        .onChange(of: viewModel.logQuery) { _, _ in
            Task {
                await viewModel.refreshLogsOnly()
            }
        }
    }

    private var filterBar: some View {
        TokfenceCard {
            HStack(spacing: 12) {
                Picker("Provider", selection: $viewModel.logProviderFilter) {
                    Text("All").tag("all")
                    ForEach(viewModel.providers, id: \.self) { provider in
                        Text(TokfenceFormatting.providerLabel(provider)).tag(provider)
                    }
                }
                .labelsHidden()
                .frame(width: 150)

                Picker("Status", selection: $viewModel.logStatusFilter) {
                    ForEach(TokfenceLogStatusFilter.allCases) { filter in
                        Text(filter.title).tag(filter)
                    }
                }
                .labelsHidden()
                .frame(width: 140)

                Picker("Range", selection: $viewModel.logTimeRange) {
                    Text("1h").tag(TokfenceTimeRange.oneHour)
                    Text("6h").tag(TokfenceTimeRange.sixHours)
                    Text("24h").tag(TokfenceTimeRange.twentyFourHours)
                    Text("7d").tag(TokfenceTimeRange.sevenDays)
                }
                .pickerStyle(.segmented)
                .frame(width: 220)

                TextField("Search model, request ID, caller", text: $viewModel.logQuery)
                    .textFieldStyle(.roundedBorder)
                    .frame(width: 220)
            }
        }
        .frame(height: 60)
    }

}

private struct BudgetSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    @State private var budgetEditor: BudgetEditorState?
    @State private var budgetToClear: String?

    private var providerBudgets: [TokfenceBudget] {
        viewModel.snapshot.budgets.filter { $0.provider != "global" }
    }

    private var spendHistory: [(day: Date, cents: Int64)] {
        let calendar = Calendar.current
        var grouped: [Date: Int64] = [:]
        for record in viewModel.logs {
            let day = calendar.startOfDay(for: record.timestamp)
            grouped[day, default: 0] += record.estimatedCostCents
        }
        return grouped.keys.sorted().map { ($0, grouped[$0, default: 0]) }
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                TokfenceSectionHeader(
                    title: "Budget",
                    subtitle: "Set spending limits and track burn",
                    trailing: AnyView(
                        Button {
                            budgetEditor = BudgetEditorState(provider: "global", period: "daily", amount: "50.00")
                        } label: {
                            Label("Edit Global", systemImage: "pencil")
                        }
                        .buttonStyle(.bordered)
                    )
                )

                globalBudgetCard
                    .frame(height: 180)

                if providerBudgets.isEmpty {
                    TokfenceEmptyState(
                        symbol: "dollarsign.circle",
                        title: "No provider budgets configured",
                        message: "Set limits to enforce daily or monthly spending boundaries.",
                        actionTitle: "Add Budget",
                        action: {
                            if let provider = viewModel.providers.first {
                                budgetEditor = BudgetEditorState(provider: provider, period: "daily", amount: "10.00")
                            }
                        }
                    )
                } else {
                    LazyVGrid(columns: [GridItem(.flexible()), GridItem(.flexible())], spacing: 12) {
                        ForEach(providerBudgets) { budget in
                            TokfenceCard {
                                HStack {
                                    Text(TokfenceFormatting.providerLabel(budget.provider))
                                        .font(.system(size: 14, weight: .semibold))
                                    Spacer()
                                    Button("Edit") {
                                        budgetEditor = BudgetEditorState(
                                            provider: budget.provider,
                                            period: budget.period,
                                            amount: String(format: "%.2f", TokfenceFormatting.usdDouble(cents: budget.limitCents))
                                        )
                                    }
                                    .buttonStyle(.bordered)

                                    Button("Clear", role: .destructive) {
                                        budgetToClear = budget.provider
                                    }
                                    .buttonStyle(.bordered)
                                }

                                Text("\(TokfenceFormatting.usd(cents: budget.currentSpendCents)) / \(TokfenceFormatting.usd(cents: budget.limitCents)) \(budget.period)")
                                    .font(.system(size: 12, weight: .medium, design: .monospaced))
                                TokfenceBudgetProgressBar(current: budget.currentSpendCents, limit: budget.limitCents)
                            }
                            .frame(height: 120)
                        }
                    }
                }

                TokfenceCard {
                    Text("Spend History")
                        .font(.system(size: 16, weight: .medium))
                    if spendHistory.isEmpty {
                        Text("Not enough data yet")
                            .font(.system(size: 12, weight: .regular))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                    } else {
                        Chart(spendHistory, id: \.day) { item in
                            BarMark(
                                x: .value("Day", item.day, unit: .day),
                                y: .value("Spend", TokfenceFormatting.usdDouble(cents: item.cents))
                            )
                            .foregroundStyle(TokfenceTheme.accentPrimary)
                        }
                        .frame(height: 180)
                    }
                }
                .frame(height: 220)
            }
        }
        .sheet(item: $budgetEditor) { state in
            BudgetEditorSheet(state: state) { amount, period in
                guard let value = Double(amount) else {
                    viewModel.lastError = "Budget amount must be numeric"
                    return
                }
                Task {
                    await viewModel.setBudget(provider: state.provider, amountUSD: value, period: period)
                    budgetEditor = nil
                }
            }
        }
        .confirmationDialog(
            "Clear budget?",
            isPresented: Binding(
                get: { budgetToClear != nil },
                set: { if !$0 { budgetToClear = nil } }
            ),
            titleVisibility: .visible
        ) {
            Button("Clear", role: .destructive) {
                guard let budgetToClear else { return }
                Task { await viewModel.clearBudget(provider: budgetToClear) }
                self.budgetToClear = nil
            }
            Button("Cancel", role: .cancel) {
                budgetToClear = nil
            }
        }
    }

    private var globalBudgetCard: some View {
        TokfenceCard {
            HStack {
                Text("Global Budget")
                    .font(.system(size: 16, weight: .medium))
                Spacer()
                if let daily = viewModel.globalDailyBudget {
                    Button("Edit") {
                        budgetEditor = BudgetEditorState(
                            provider: "global",
                            period: "daily",
                            amount: String(format: "%.2f", TokfenceFormatting.usdDouble(cents: daily.limitCents))
                        )
                    }
                    .buttonStyle(.bordered)
                }
            }

            if let daily = viewModel.globalDailyBudget {
                Text("\(TokfenceFormatting.usd(cents: daily.currentSpendCents)) / \(TokfenceFormatting.usd(cents: daily.limitCents)) daily")
                    .font(.system(size: 20, weight: .semibold, design: .monospaced))
                TokfenceBudgetProgressBar(current: daily.currentSpendCents, limit: daily.limitCents)
                Text("Projected: \(TokfenceFormatting.usd(cents: TokfenceFormatting.projectedSpend(currentCents: daily.currentSpendCents, periodStart: daily.periodStart, period: daily.period)))")
                    .font(.system(size: 12, weight: .regular, design: .monospaced))
                    .foregroundStyle(TokfenceTheme.textSecondary)
            } else {
                Text("No global daily budget configured")
                    .font(.system(size: 12, weight: .regular))
                    .foregroundStyle(TokfenceTheme.textSecondary)
            }

            if let monthly = viewModel.globalMonthlyBudget {
                Divider()
                Text("Monthly: \(TokfenceFormatting.usd(cents: monthly.currentSpendCents)) / \(TokfenceFormatting.usd(cents: monthly.limitCents))")
                    .font(.system(size: 12, weight: .medium, design: .monospaced))
                    .foregroundStyle(TokfenceTheme.textPrimary)
            }
        }
    }
}

private struct ProvidersSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    @State private var providerToRevoke: String?
    @State private var providerToRestore: String?
    @State private var rateEditor: RateLimitEditorState?
    @State private var confirmKillSwitch = false

    private enum ProviderOperationalStatus {
        case active
        case revoked
        case missingKey
    }

    private func operationalStatus(for provider: TokfenceProviderOverview) -> ProviderOperationalStatus {
        if !provider.hasKey {
            return .missingKey
        }
        if provider.isRevoked {
            return .revoked
        }
        return .active
    }

    private func statusDotColor(for status: ProviderOperationalStatus) -> Color {
        switch status {
        case .active:
            return TokfenceTheme.healthy
        case .revoked:
            return TokfenceTheme.warning
        case .missingKey:
            return TokfenceTheme.textTertiary
        }
    }

    private func statusLabel(for status: ProviderOperationalStatus) -> String {
        switch status {
        case .active:
            return "Active"
        case .revoked:
            return "Revoked"
        case .missingKey:
            return "Missing Key"
        }
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                TokfenceSectionHeader(
                    title: "Providers",
                    subtitle: "Manage provider access and limits"
                )

                TokfenceCard {
                    HStack(alignment: .center) {
                        VStack(alignment: .leading, spacing: 4) {
                            Text("Emergency Kill Switch")
                                .font(.system(size: 15, weight: .semibold))
                            Text("Immediately blocks all API requests.")
                                .font(.system(size: 12, weight: .regular))
                                .foregroundStyle(TokfenceTheme.textSecondary)
                        }
                        Spacer()
                        Button(viewModel.snapshot.killSwitchActive ? "Deactivate Kill Switch" : "Activate Kill Switch") {
                            confirmKillSwitch = true
                        }
                        .buttonStyle(.borderedProminent)
                        .tint(viewModel.snapshot.killSwitchActive ? TokfenceTheme.healthy : TokfenceTheme.danger)
                    }
                }
                .frame(height: 120)

                ForEach(viewModel.providerOverview) { provider in
                    let status = operationalStatus(for: provider)
                    TokfenceCard {
                        HStack {
                            Circle()
                                .fill(statusDotColor(for: status))
                                .frame(width: 10, height: 10)
                            Text(TokfenceFormatting.providerLabel(provider.provider))
                                .font(.system(size: 15, weight: .semibold))
                            Spacer()
                            HStack(spacing: 8) {
                                Text(statusLabel(for: status))
                                    .font(.system(size: 11, weight: .medium))
                                    .padding(.vertical, 4)
                                    .padding(.horizontal, 8)
                                    .background(TokfenceTheme.bgTertiary, in: RoundedRectangle(cornerRadius: TokfenceTheme.badgeCorner, style: .continuous))
                                if status == .revoked {
                                    Button("Restore") {
                                        providerToRestore = provider.provider
                                    }
                                    .buttonStyle(.borderedProminent)
                                    .tint(TokfenceTheme.healthy)
                                } else if status == .active {
                                    Button("Revoke") {
                                        providerToRevoke = provider.provider
                                    }
                                    .buttonStyle(.bordered)
                                } else {
                                    Button("Configure Key") {
                                        withAnimation(TokfenceTheme.uiSpring) {
                                            viewModel.selectedSection = .vault
                                        }
                                    }
                                    .buttonStyle(.bordered)
                                }
                            }
                        }

                        Text("Endpoint: \(provider.upstream)")
                            .font(.system(size: 11, weight: .regular, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textSecondary)

                        Text("Key: \(provider.hasKey ? "configured" : "missing")")
                            .font(.system(size: 12, weight: .medium))
                            .foregroundStyle(provider.hasKey ? TokfenceTheme.textPrimary : TokfenceTheme.warning)

                        HStack(spacing: 8) {
                            Text("Rate limit: \(provider.rateLimitRPM.map { "\($0) RPM" } ?? "not set")")
                                .font(.system(size: 12, weight: .regular))
                                .foregroundStyle(TokfenceTheme.textSecondary)
                            Button("Edit") {
                                rateEditor = RateLimitEditorState(provider: provider.provider, rpm: provider.rateLimitRPM.map(String.init) ?? "60")
                            }
                            .buttonStyle(.bordered)
                            if provider.rateLimitRPM != nil {
                                Button("Clear") {
                                    Task { await viewModel.clearRateLimit(provider: provider.provider) }
                                }
                                .buttonStyle(.bordered)
                            }
                        }

                        Text("Today: \(provider.todayRequests) requests · \(TokfenceFormatting.tokens(provider.todayInputTokens + provider.todayOutputTokens)) tokens · \(TokfenceFormatting.usd(cents: provider.todayCostCents))")
                            .font(.system(size: 12, weight: .medium, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textPrimary)
                    }
                    .opacity(status == .active ? 1 : 0.84)
                    .frame(height: 160)
                }
            }
        }
        .sheet(item: $rateEditor) { state in
            RateLimitEditorSheet(state: state) { rpm in
                guard let rpmValue = Int(rpm), rpmValue > 0 else {
                    viewModel.lastError = "RPM must be a positive integer"
                    return
                }
                Task {
                    await viewModel.setRateLimit(provider: state.provider, rpm: rpmValue)
                    rateEditor = nil
                }
            }
        }
        .confirmationDialog(
            "Change provider state",
            isPresented: Binding(
                get: { providerToRevoke != nil || providerToRestore != nil },
                set: { if !$0 { providerToRevoke = nil; providerToRestore = nil } }
            )
        ) {
            if let providerToRevoke {
                Button("Revoke \(TokfenceFormatting.providerLabel(providerToRevoke))", role: .destructive) {
                    Task { await viewModel.revokeProvider(providerToRevoke) }
                    self.providerToRevoke = nil
                }
            }
            if let providerToRestore {
                Button("Restore \(TokfenceFormatting.providerLabel(providerToRestore))") {
                    Task { await viewModel.restoreProvider(providerToRestore) }
                    self.providerToRestore = nil
                }
            }
            Button("Cancel", role: .cancel) {
                providerToRevoke = nil
                providerToRestore = nil
            }
        }
        .confirmationDialog(
            viewModel.snapshot.killSwitchActive ? "Deactivate kill switch?" : "Activate kill switch?",
            isPresented: $confirmKillSwitch,
            titleVisibility: .visible
        ) {
            if viewModel.snapshot.killSwitchActive {
                Button("Deactivate") {
                    Task { await viewModel.killSwitchOff() }
                }
            } else {
                Button("Activate", role: .destructive) {
                    Task { await viewModel.killSwitchOn() }
                }
            }
            Button("Cancel", role: .cancel) {}
        }
    }
}

// MARK: - Agents Section (Card-per-Agent)

private struct AgentsSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    // Agent config (will become dynamic with multiple agents)
    @State private var image = "ghcr.io/openclaw/openclaw:latest"
    @State private var containerName = "tokfence-openclaw"
    @State private var gatewayPort = "18789"
    @State private var workspace = "~/openclaw/workspace"
    @State private var noPull = false
    @State private var openDashboard = true
    @State private var showConfig = false
    @State private var selectedRequestID: String? = nil
    @State private var setupReadyPulse = false
    @State private var previousSetupComplete = false
    @State private var isOpeningAgentTarget = false

    private var agent: TokfenceAgentCardModel { viewModel.primaryAgentCard }
    private var shouldShowGuidedSetup: Bool {
        !viewModel.isSetupComplete && agent.status != .running && agent.status != .starting
    }
    private var shouldShowSetupReady: Bool {
        viewModel.isSetupComplete && agent.status == .stopped
    }
    private var openActionsDisabled: Bool {
        isOpeningAgentTarget || viewModel.launchBusy
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 20) {
                TokfenceSectionHeader(
                    title: "Agents",
                    subtitle: "Your AI tools, running securely"
                )

                primaryAgentCard

                ForEach(viewModel.placeholderAgentCards) { placeholder in
                    placeholderCard(placeholder)
                }

                TokfenceCard {
                    HStack(spacing: 8) {
                        Image(systemName: "lock.shield")
                            .font(.system(size: 12, weight: .semibold))
                            .foregroundStyle(TokfenceTheme.accentPrimary)
                        Text("Security: API keys remain in your encrypted vault. Agents never see real credentials.")
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                    }
                }
            }
            .padding(.bottom, 8)
            .animation(TokfenceTheme.uiSpring, value: agent.status)
        }
        .onAppear {
            previousSetupComplete = viewModel.isSetupComplete
            Task { await viewModel.refreshLaunchState() }
        }
        .onChange(of: viewModel.isSetupComplete) { _, isNowComplete in
            if isNowComplete && !previousSetupComplete {
                withAnimation(TokfenceTheme.uiSpring) {
                    setupReadyPulse = true
                }
                Task {
                    try? await Task.sleep(nanoseconds: 1_000_000_000)
                    await MainActor.run {
                        withAnimation(TokfenceTheme.uiAnimation) {
                            setupReadyPulse = false
                        }
                    }
                }
            }
            previousSetupComplete = isNowComplete
        }
        .sheet(isPresented: $showConfig) {
            agentConfigSheet
        }
    }

    private var primaryAgentCard: some View {
        TokfenceCard {
            HStack(alignment: .center, spacing: 12) {
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(TokfenceTheme.info.opacity(0.15))
                    .frame(width: 44, height: 44)
                    .overlay {
                        Image(systemName: "hammer.fill")
                            .font(.system(size: 20, weight: .medium))
                            .foregroundStyle(TokfenceTheme.info)
                    }

                VStack(alignment: .leading, spacing: 3) {
                    Text(agent.name)
                        .font(.system(size: 17, weight: .semibold))
                        .foregroundStyle(TokfenceTheme.textPrimary)
                    Text(agent.subtitle)
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }

                Spacer(minLength: 0)

                statusBadge

                Button {
                    showConfig = true
                } label: {
                    Image(systemName: "gearshape")
                }
                .buttonStyle(.bordered)
                .help("Configure OpenClaw")
            }

            switch agent.status {
            case .running:
                runningAgentContent
            case .starting:
                startingAgentContent
            case .error:
                errorAgentContent
            case .stopped, .placeholder:
                if shouldShowGuidedSetup {
                    guidedSetupContent
                } else if shouldShowSetupReady {
                    setupReadyContent
                } else {
                    stoppedAgentContent
                }
            }
        }
        .overlay(alignment: .leading) {
            if agent.status == .running || agent.status == .error {
                RoundedRectangle(cornerRadius: 2, style: .continuous)
                    .fill(agent.status == .running ? TokfenceTheme.healthy : TokfenceTheme.danger)
                    .frame(width: 4)
                    .padding(.vertical, 8)
            }
        }
    }

    private var statusBadge: some View {
        Group {
            switch agent.status {
            case .starting:
                HStack(spacing: 6) {
                    ProgressView()
                        .controlSize(.small)
                    Text("Starting")
                        .font(.system(size: 12, weight: .medium))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }
            case .running:
                TokfenceLiveBadge(text: "Running", color: TokfenceTheme.healthy, isActive: true)
            case .error:
                TokfenceLiveBadge(text: "Error", color: TokfenceTheme.danger, isActive: false)
            case .stopped, .placeholder:
                Text("Stopped")
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(TokfenceTheme.textTertiary)
            }
        }
    }

    private var stoppedAgentContent: some View {
        VStack(alignment: .leading, spacing: 12) {
            Button {
                Task { await startAgent() }
            } label: {
                Label("Start", systemImage: "play.fill")
                    .frame(maxWidth: .infinity)
            }
            .accessibilityIdentifier("agents.start")
            .buttonStyle(.borderedProminent)
            .tint(TokfenceTheme.healthy)
            .frame(minHeight: 44)
            .disabled(viewModel.launchBusy)
        }
    }

    private var guidedSetupContent: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Guided Setup")
                .font(.system(size: 14, weight: .semibold))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text("Tokfence will set up a security-first OpenClaw environment step by step.")
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)

            ForEach(viewModel.setupSteps) { step in
                setupStepRow(step)
            }

            if let reason = viewModel.setupBlockingReason {
                Text(reason)
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(TokfenceTheme.textSecondary)
                    .textSelection(.enabled)
            }
        }
    }

    private var setupReadyContent: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                Image(systemName: "checkmark.seal.fill")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.healthy)
                Text("Setup complete. OpenClaw is ready to start.")
                    .font(.system(size: 13, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.textPrimary)
            }
            .padding(.horizontal, 10)
            .padding(.vertical, 8)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .fill(TokfenceTheme.healthy.opacity(setupReadyPulse ? 0.22 : 0.12))
            )
            .overlay(
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .stroke(TokfenceTheme.healthy.opacity(setupReadyPulse ? 0.85 : 0.45), lineWidth: setupReadyPulse ? 1.4 : 1)
            )
            .scaleEffect(setupReadyPulse ? 1.01 : 1.0)

            Button {
                Task { await startAgent() }
            } label: {
                Label("Start OpenClaw", systemImage: "play.fill")
                    .frame(maxWidth: .infinity)
            }
            .accessibilityIdentifier("agents.startOpenClaw")
            .buttonStyle(.borderedProminent)
            .tint(TokfenceTheme.healthy)
            .frame(minHeight: 44)
            .disabled(viewModel.launchBusy)
        }
        .animation(TokfenceTheme.uiSpring, value: setupReadyPulse)
    }

    private var startingAgentContent: some View {
        HStack(spacing: 10) {
            ProgressView()
                .progressViewStyle(.circular)
            Text("Pulling image and starting container...")
                .font(.system(size: 13, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
        .padding(.vertical, 6)
    }

    private var runningAgentContent: some View {
        VStack(alignment: .leading, spacing: 12) {
            if !agent.uptimeText.isEmpty || !agent.gatewayURL.isEmpty {
                HStack(spacing: 12) {
                    if !agent.uptimeText.isEmpty {
                        agentInfoPill(icon: "clock", text: agent.uptimeText)
                    }
                    if !agent.gatewayURL.isEmpty {
                        agentInfoPill(icon: "link", text: agent.gatewayURL)
                            .onTapGesture { copyToClipboard(agent.gatewayURL) }
                    }
                }
            }

            HStack(spacing: 10) {
                Button {
                    Task {
                        await openDashboardTapped()
                    }
                } label: {
                    Label("Open OpenClaw", systemImage: "rectangle.and.cursor.arrow")
                        .frame(maxWidth: .infinity)
                }
                .accessibilityIdentifier("agents.openDashboard")
                .overlay {
                    if isOpeningAgentTarget {
                        ProgressView()
                            .controlSize(.small)
                            .tint(.white)
                    }
                }
                .buttonStyle(.borderedProminent)
                .tint(TokfenceTheme.info)
                .disabled(openActionsDisabled)

                Button {
                    Task {
                        await openGatewayTapped()
                    }
                } label: {
                    Label("Go to OpenClaw", systemImage: "arrow.up.right.square")
                        .frame(maxWidth: .infinity)
                }
                .accessibilityIdentifier("agents.openGateway")
                .overlay {
                    if isOpeningAgentTarget {
                        ProgressView()
                            .controlSize(.small)
                    }
                }
                .buttonStyle(.bordered)
                .disabled(openActionsDisabled)
            }

            HStack(spacing: 10) {
                Button {
                    Task { await viewModel.launchStop() }
                } label: {
                    Label("Stop", systemImage: "stop.fill")
                        .frame(maxWidth: .infinity)
                }
                .accessibilityIdentifier("agents.stop")
                .buttonStyle(.bordered)
                .tint(TokfenceTheme.danger)
                .disabled(viewModel.launchBusy)

                Button {
                    Task { await viewModel.launchRestart() }
                } label: {
                    Label("Restart", systemImage: "arrow.clockwise")
                        .frame(maxWidth: .infinity)
                }
                .accessibilityIdentifier("agents.restart")
                .buttonStyle(.bordered)
                .disabled(viewModel.launchBusy)
            }

            if !agent.providers.isEmpty {
                HStack(spacing: 6) {
                    ForEach(agent.providers, id: \.self) { provider in
                        TokfenceProviderBadge(provider: provider, active: true)
                    }
                }
            }

            if !agent.recentActivity.isEmpty {
                Divider()
                Text("Recent activity")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.textSecondary)
                RequestListPanel(records: agent.recentActivity, selectedRequestID: $selectedRequestID, compact: true)
                HStack {
                    Spacer()
                    Button("View all activity") {
                        withAnimation(TokfenceTheme.uiSpring) {
                            viewModel.selectedSection = .activity
                        }
                    }
                    .buttonStyle(.link)
                    .font(.system(size: 11, weight: .medium))
                }
            }
        }
    }

    private var errorAgentContent: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text(agent.lastError.isEmpty ? "OpenClaw failed to start." : agent.lastError)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.danger)
                .textSelection(.enabled)

            HStack(spacing: 10) {
                Button {
                    Task { await startAgent() }
                } label: {
                    Label("Retry", systemImage: "arrow.clockwise")
                        .frame(maxWidth: .infinity)
                }
                .accessibilityIdentifier("agents.retryStart")
                .buttonStyle(.borderedProminent)
                .tint(TokfenceTheme.warning)
                .disabled(viewModel.launchBusy)

                Button {
                    showConfig = true
                } label: {
                    Label("Configure", systemImage: "slider.horizontal.3")
                        .frame(maxWidth: .infinity)
                }
                .buttonStyle(.bordered)
            }
        }
    }

    private func placeholderCard(_ card: TokfenceAgentCardModel) -> some View {
        TokfenceCard {
            HStack(spacing: 12) {
                RoundedRectangle(cornerRadius: 10, style: .continuous)
                    .fill(TokfenceTheme.bgTertiary)
                    .frame(width: 40, height: 40)
                    .overlay {
                        Image(systemName: "clock.badge")
                            .font(.system(size: 16, weight: .semibold))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                    }
                VStack(alignment: .leading, spacing: 2) {
                    Text(card.name)
                        .font(.system(size: 14, weight: .semibold))
                        .foregroundStyle(TokfenceTheme.textPrimary)
                    Text(card.subtitle)
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }
                Spacer()
                Text("Coming soon")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.textTertiary)
            }
            .opacity(0.78)
        }
    }

    // MARK: - Config Sheet

    private var agentConfigSheet: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("OpenClaw Configuration")
                .font(.system(size: 16, weight: .semibold))
                .foregroundStyle(TokfenceTheme.textPrimary)

            TextField("Docker image", text: $image)
                .textFieldStyle(.roundedBorder)
            TextField("Container name", text: $containerName)
                .textFieldStyle(.roundedBorder)
            TextField("Gateway port", text: $gatewayPort)
                .textFieldStyle(.roundedBorder)
            TextField("Workspace path", text: $workspace)
                .textFieldStyle(.roundedBorder)
            Toggle("Skip image pull", isOn: $noPull)
                .toggleStyle(.switch)
            Toggle("Open dashboard after start", isOn: $openDashboard)
                .toggleStyle(.switch)

            HStack {
                Spacer()
                Button("Done") { showConfig = false }
                    .buttonStyle(.borderedProminent)
                    .tint(TokfenceTheme.accentPrimary)
            }
        }
        .padding(16)
            .frame(minWidth: 420)
    }

    // MARK: - Helpers

    private func setupStepRow(_ step: TokfenceSetupStepState) -> some View {
        VStack(alignment: .leading, spacing: 8) {
            HStack(spacing: 8) {
                Circle()
                    .fill(setupStepColor(step.status))
                    .frame(width: 8, height: 8)
                Text(step.title)
                    .font(.system(size: 12, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.textPrimary)
                Spacer(minLength: 0)
                Text(setupStepStatusLabel(step.status))
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(TokfenceTheme.textSecondary)
            }

            Text(step.reason)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.textPrimary.opacity(0.92))
                .lineSpacing(1.5)

            switch step.status {
            case .failed(let message):
                Text(message)
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(TokfenceTheme.danger)
                    .lineLimit(2)
                    .textSelection(.enabled)
            default:
                EmptyView()
            }

            HStack {
                Spacer(minLength: 0)
                setupStepTrailingControl(step)
            }
        }
        .padding(12)
        .background(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .fill(TokfenceTheme.bgTertiary.opacity(0.5))
        )
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(stepRowStrokeColor(step.status), lineWidth: 1)
        )
    }

    @ViewBuilder
    private func setupStepTrailingControl(_ step: TokfenceSetupStepState) -> some View {
        if let actionTitle = step.actionTitle {
            Button {
                Task { await runSetupAction(step.id) }
            } label: {
                Label(actionTitle, systemImage: setupStepActionIcon(step.id))
                    .frame(width: 190, alignment: .leading)
            }
            .buttonStyle(.bordered)
            .disabled(!step.isActionEnabled || viewModel.launchBusy)
        } else {
            HStack(spacing: 6) {
                Image(systemName: "checkmark.circle.fill")
                    .foregroundStyle(TokfenceTheme.healthy)
                Text("Completed")
            }
            .font(.system(size: 11, weight: .semibold))
            .foregroundStyle(TokfenceTheme.textSecondary)
            .frame(width: 190, alignment: .leading)
        }
    }

    private func setupStepActionIcon(_ id: TokfenceSetupStepID) -> String {
        switch id {
        case .docker:
            return "arrow.up.right.square"
        case .daemon:
            return "play.circle.fill"
        case .vault:
            return "key.fill"
        case .container:
            return "hammer.fill"
        }
    }

    private func stepRowStrokeColor(_ status: TokfenceSetupStepStatus) -> Color {
        switch status {
        case .complete:
            return TokfenceTheme.healthy.opacity(0.28)
        case .failed:
            return TokfenceTheme.danger.opacity(0.35)
        case .inProgress:
            return TokfenceTheme.info.opacity(0.35)
        case .pending:
            return TokfenceTheme.bgTertiary.opacity(0.4)
        }
    }

    private func setupStepColor(_ status: TokfenceSetupStepStatus) -> Color {
        switch status {
        case .complete:
            return TokfenceTheme.healthy
        case .inProgress:
            return TokfenceTheme.info
        case .failed:
            return TokfenceTheme.danger
        case .pending:
            return TokfenceTheme.textTertiary
        }
    }

    private func setupStepStatusLabel(_ status: TokfenceSetupStepStatus) -> String {
        switch status {
        case .complete:
            return "Done"
        case .inProgress:
            return "In progress"
        case .failed:
            return "Needs attention"
        case .pending:
            return "Pending"
        }
    }

    private func runSetupAction(_ id: TokfenceSetupStepID) async {
        switch id {
        case .docker:
            openDockerAction()
        case .daemon:
            await viewModel.startDaemon()
        case .vault:
            withAnimation(TokfenceTheme.uiSpring) {
                viewModel.selectedSection = .vault
            }
        case .container:
            await startAgent()
        }
    }

    private func openDockerAction() {
        let errorText = viewModel.launchStatusError.lowercased()
        if errorText.contains("not installed") || errorText.contains("binary not found") {
            if let url = URL(string: "https://docker.com/get-started") {
                let opened = openURLOrFallback(url)
                if !opened {
                    viewModel.surfaceError("Could not open Docker download page.")
                }
            }
            return
        }

        let dockerAppURL = URL(fileURLWithPath: "/Applications/Docker.app")
        if FileManager.default.fileExists(atPath: dockerAppURL.path) {
            NSWorkspace.shared.open(dockerAppURL)
            return
        }
        if let url = URL(string: "https://docker.com/get-started") {
            let opened = openURLOrFallback(url)
            if !opened {
                viewModel.surfaceError("Could not open Docker app or documentation URL.")
            }
        }
    }

    private func agentInfoPill(icon: String, text: String) -> some View {
        HStack(spacing: 5) {
            Image(systemName: icon)
                .font(.system(size: 10, weight: .medium))
            Text(text)
                .font(.system(size: 11, weight: .medium, design: .monospaced))
        }
        .foregroundStyle(TokfenceTheme.textSecondary)
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(TokfenceTheme.bgTertiary, in: RoundedRectangle(cornerRadius: 4, style: .continuous))
    }

    private func startAgent() async {
        await viewModel.launchStart(
            image: image,
            name: containerName,
            portText: gatewayPort,
            workspace: workspace,
            noPull: noPull,
            openDashboard: openDashboard
        )
        if openDashboard {
            await openAgentDashboard()
        }
    }

    private func openDashboardTapped() async {
        await performOpenAction {
            await openAgentDashboard()
        }
    }

    private func openGatewayTapped() async {
        await performOpenAction {
            _ = await openAgentGateway()
        }
    }

    private func performOpenAction(_ action: @escaping () async -> Void) async {
        if isOpeningAgentTarget {
            return
        }
        isOpeningAgentTarget = true
        defer { isOpeningAgentTarget = false }
        await action()
    }

    private func openAgentDashboard() async {
        await refreshLaunchStateForOpen()
        if let url = resolvedDashboardURL(), let resolved = URL(string: url) {
            if openURLOrFallback(resolved) {
                return
            }
            viewModel.surfaceError("Could not open dashboard URL: \(url)")
            return
        }
        if await openAgentGateway() {
            return
        }
        viewModel.surfaceError("OpenClaw dashboard is not available. Check container status or gateway port.")
    }

    private func openAgentGateway() async -> Bool {
        await refreshLaunchStateForOpen()
        guard let url = resolvedGatewayURL(), !url.isEmpty else {
            viewModel.surfaceError("Gateway URL is not available. Start OpenClaw first.")
            return false
        }
        guard let resolved = URL(string: url) else {
            viewModel.surfaceError("Gateway URL is malformed.")
            return false
        }
        if openURLOrFallback(resolved) {
            return true
        }
        viewModel.surfaceError("Could not open gateway URL: \(url)")
        return false
    }

    private func refreshLaunchStateForOpen() async {
        for attempt in 0..<8 {
            if !viewModel.launchBusy {
                await viewModel.launchStatus()
            }
            if !viewModel.launchBusy && viewModel.primaryAgentCard.status == .running {
                return
            }
            if !viewModel.launchResult.configPath.isEmpty,
               (!viewModel.primaryAgentCard.dashboardURL.isEmpty || !viewModel.primaryAgentCard.gatewayURL.isEmpty) {
                return
            }
            if attempt < 7 {
                try? await Task.sleep(for: .milliseconds(300))
            }
        }
    }

    private func copyToClipboard(_ value: String) {
        let trimmed = value.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !trimmed.isEmpty else { return }
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(trimmed, forType: .string)
    }

    private func resolvedGatewayURL() -> String? {
        let trimmed = agent.gatewayURL.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.isEmpty ? nil : trimmed
    }

    private func resolvedDashboardURL() -> String? {
        let trimmedDashboard = agent.dashboardURL.trimmingCharacters(in: .whitespacesAndNewlines)
        if !trimmedDashboard.isEmpty {
            return trimmedDashboard
        }
        guard let gateway = resolvedGatewayURL() else {
            return nil
        }
        let trimmedToken = agent.gatewayToken.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmedToken.isEmpty {
            return gateway
        }
        if gateway.contains("?") {
            return "\(gateway)&token=\(trimmedToken)"
        }
        return "\(gateway)/?token=\(trimmedToken)"
    }

    private func openURLOrFallback(_ url: URL) -> Bool {
        return NSWorkspace.shared.open(url)
    }
}

private struct SettingsSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    @State private var shellSnippet = ""

    private var sortedBaseURLs: [(key: String, value: String)] {
        viewModel.envMap
            .filter { $0.key.hasSuffix("_BASE_URL") }
            .sorted { $0.key < $1.key }
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Settings")
                .font(.system(size: 20, weight: .semibold))
                .foregroundStyle(TokfenceTheme.textPrimary)

            TokfenceCard {
                VStack(alignment: .leading, spacing: 12) {
                    settingsTitle("Daemon")
                    settingsValue("Port 9471 · Auto-start: On · Bind \(viewModel.snapshot.addr ?? "127.0.0.1:9471")")

                    settingsTitle("Vault")
                    settingsValue("Backend: macOS Keychain · Change passphrase")

                    settingsTitle("Logging")
                    settingsValue("DB: ~/.tokfence/tokfence.db · Retention 30d")

                    settingsTitle("Notifications")
                    settingsValue("Warn at 80% · macOS notifications On")

                    settingsTitle("Shell")
                    settingsValue("eval \"$(tokfence env)\"")

                    settingsTitle("Base URLs")
                    if sortedBaseURLs.isEmpty {
                        settingsValue("No BASE_URL exports available")
                    } else {
                        ForEach(sortedBaseURLs, id: \.key) { item in
                            Text("\(item.key)=\(item.value)")
                                .font(.system(size: 11, weight: .medium, design: .monospaced))
                                .foregroundStyle(TokfenceTheme.textSecondary)
                                .textSelection(.enabled)
                        }
                    }

                    settingsTitle("About")
                    settingsValue("Version \(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "1.0.0") · Build 2026-02-16 · GitHub · Docs")
                }
            }
            .frame(height: 500)

            HStack(spacing: 8) {
                Button("Start") { Task { await viewModel.startDaemon() } }
                    .buttonStyle(.bordered)
                Button("Stop") { Task { await viewModel.stopDaemon() } }
                    .buttonStyle(.bordered)
                Button("Refresh") { Task { await viewModel.refreshAll() } }
                    .buttonStyle(.bordered)
                Button("Save Binary Path") { viewModel.saveBinaryPath() }
                    .buttonStyle(.bordered)
                Button("Open Data Folder") { viewModel.openDataFolder() }
                    .buttonStyle(.bordered)
                Button("Copy Shell Snippet") {
                    let value = shellSnippet.isEmpty ? viewModel.shellSnippet() : shellSnippet
                    NSPasteboard.general.clearContents()
                    NSPasteboard.general.setString(value, forType: .string)
                }
                .buttonStyle(.bordered)
            }

            TextField("Tokfence binary path", text: $viewModel.binaryPath)
                .textFieldStyle(.roundedBorder)
                .frame(maxWidth: 460)
        }
    }

    private func settingsTitle(_ text: String) -> some View {
        Text(text)
            .font(.system(size: 13, weight: .semibold))
            .foregroundStyle(TokfenceTheme.textPrimary)
    }

    private func settingsValue(_ text: String) -> some View {
        Text(text)
            .font(.system(size: 12, weight: .regular))
            .foregroundStyle(TokfenceTheme.textSecondary)
    }
}

private struct LogDetailPanel: View {
    let record: TokfenceLogRecord

    var body: some View {
        VStack(alignment: .leading, spacing: 10) {
            Text("Request Detail")
                .font(.system(size: 16, weight: .semibold))
            detailRow("Request ID", record.id)
            detailRow("Timestamp", TokfenceFormatting.shortDateTime(record.timestamp))
            detailRow("Provider", TokfenceFormatting.providerLabel(record.provider))
            detailRow("Model", record.model)
            detailRow("Endpoint", record.endpoint)
            detailRow("Method", record.method)
            detailRow("Input Tokens", "\(record.inputTokens)")
            detailRow("Output Tokens", "\(record.outputTokens)")
            detailRow("Cost", TokfenceFormatting.usd(cents: record.estimatedCostCents))
            detailRow("Status", "\(record.statusCode)")
            detailRow("TTFB", TokfenceFormatting.latency(ms: record.ttftMS))
            detailRow("Latency", TokfenceFormatting.latency(ms: record.latencyMS))
            detailRow("Caller", record.callerName.isEmpty ? "unknown" : "\(record.callerName) (pid \(record.callerPID))")
            detailRow("Streaming", record.isStreaming ? "yes" : "no")

            if !record.errorType.isEmpty || !record.errorMessage.isEmpty {
                Divider()
                Text("Error")
                    .font(.system(size: 13, weight: .semibold))
                Text("\(record.errorType): \(record.errorMessage)")
                    .font(.system(size: 12, weight: .regular, design: .monospaced))
                    .foregroundStyle(TokfenceTheme.danger)
            }

            Spacer(minLength: 0)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func detailRow(_ label: String, _ value: String) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
            Text(label)
                .font(.system(size: 11, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
                .frame(width: 96, alignment: .leading)
            Text(value.isEmpty ? "-" : value)
                .font(.system(size: 11, weight: .medium, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
                .textSelection(.enabled)
            Spacer(minLength: 0)
        }
    }
}

private struct VaultKeySubmission {
    let provider: String
    let key: String
    let endpoint: String?
    let model: String?
}

private struct VaultKeyEditorState: Identifiable {
    enum Mode {
        case add
        case rotate
    }

    let id = UUID()
    let provider: String
    let mode: Mode
    let upstream: String
}

private struct VaultKeyEditorSheet: View {
    private struct PopularProviderPreset: Identifiable, Hashable {
        let id: String
        let company: String
        let endpoint: String
        let models: [String]
    }

    let state: VaultKeyEditorState
    let knownUpstreams: [String: String]
    let onSubmit: (VaultKeySubmission) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var selectedProvider = "anthropic"
    @State private var customProvider = ""
    @State private var endpoint = ""
    @State private var model = ""
    @State private var key = ""
    @State private var endpointManuallyEdited = false
    @State private var suppressEndpointTracking = false
    @State private var modelManuallyEdited = false

    private static let customProviderOption = "__custom__"
    private static let popularProviderPresets: [PopularProviderPreset] = [
        PopularProviderPreset(
            id: "anthropic",
            company: "Anthropic",
            endpoint: "https://api.anthropic.com",
            models: ["claude-sonnet-4-5", "claude-3-5-haiku-latest", "claude-3-5-sonnet-latest"]
        ),
        PopularProviderPreset(
            id: "openai",
            company: "OpenAI",
            endpoint: "https://api.openai.com",
            models: ["gpt-5.1", "gpt-4o", "gpt-4o-mini"]
        ),
        PopularProviderPreset(
            id: "google",
            company: "Google (Gemini)",
            endpoint: "https://generativelanguage.googleapis.com",
            models: ["gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"]
        ),
        PopularProviderPreset(
            id: "mistral",
            company: "Mistral",
            endpoint: "https://api.mistral.ai",
            models: ["mistral-large-latest", "mistral-medium-latest", "mistral-small-latest"]
        ),
        PopularProviderPreset(
            id: "groq",
            company: "Groq",
            endpoint: "https://api.groq.com/openai",
            models: ["llama-3.1-8b-instant", "llama-3.3-70b-versatile", "mixtral-8x7b-32768"]
        ),
        PopularProviderPreset(
            id: "openrouter",
            company: "OpenRouter",
            endpoint: "https://openrouter.ai/api",
            models: ["openrouter/auto", "anthropic/claude-3.5-sonnet", "openai/gpt-4o-mini"]
        ),
        PopularProviderPreset(
            id: "deepseek",
            company: "DeepSeek",
            endpoint: "https://api.deepseek.com",
            models: ["deepseek-chat", "deepseek-reasoner", "deepseek-coder"]
        ),
        PopularProviderPreset(
            id: "together",
            company: "Together AI",
            endpoint: "https://api.together.xyz/v1",
            models: ["meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo", "Qwen/Qwen2.5-72B-Instruct-Turbo", "mistralai/Mixtral-8x7B-Instruct-v0.1"]
        ),
        PopularProviderPreset(
            id: "cohere",
            company: "Cohere",
            endpoint: "https://api.cohere.com/v1",
            models: ["command-r-plus", "command-r", "command"]
        ),
        PopularProviderPreset(
            id: "perplexity",
            company: "Perplexity",
            endpoint: "https://api.perplexity.ai",
            models: ["sonar", "sonar-pro", "sonar-reasoning"]
        ),
    ]

    private var selectedPreset: PopularProviderPreset? {
        Self.popularProviderPresets.first(where: { $0.id == selectedProvider })
    }

    private var normalizedCustomProvider: String {
        customProvider.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    }

    private var normalizedProvider: String {
        if state.mode == .rotate {
            return state.provider
        }
        if selectedProvider == Self.customProviderOption {
            return normalizedCustomProvider
        }
        return selectedProvider
    }

    private var trimmedKey: String {
        key.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedEndpoint: String {
        endpoint.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedModel: String {
        model.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var modelOptions: [String] {
        selectedPreset?.models ?? []
    }

    private var isValidProviderName: Bool {
        guard !normalizedProvider.isEmpty else { return false }
        return normalizedProvider.range(of: "^[a-z0-9][a-z0-9_-]{0,62}$", options: .regularExpression) != nil
    }

    private var suggestedEndpoint: String? {
        guard !normalizedProvider.isEmpty else { return nil }
        if let existing = knownUpstreams[normalizedProvider], !existing.isEmpty {
            return existing
        }
        if let preset = selectedPreset {
            return preset.endpoint
        }
        return nil
    }

    private var suggestedModel: String? {
        modelOptions.first
    }

    private var resolvedEndpoint: String? {
        if !trimmedEndpoint.isEmpty {
            return trimmedEndpoint
        }
        return suggestedEndpoint
    }

    private var resolvedModel: String? {
        if !trimmedModel.isEmpty {
            return trimmedModel
        }
        return suggestedModel
    }

    private var requiresEndpoint: Bool {
        state.mode == .add && suggestedEndpoint == nil
    }

    private var canSubmit: Bool {
        switch state.mode {
        case .add:
            if !isValidProviderName || trimmedKey.isEmpty {
                return false
            }
            if requiresEndpoint && resolvedEndpoint == nil {
                return false
            }
            if resolvedModel == nil {
                return false
            }
            return true
        case .rotate:
            return !trimmedKey.isEmpty
        }
    }

    private func applyEndpointSuggestionIfNeeded() {
        guard state.mode == .add else { return }
        if endpointManuallyEdited && !trimmedEndpoint.isEmpty {
            return
        }
        let value = suggestedEndpoint ?? ""
        suppressEndpointTracking = true
        endpoint = value
        suppressEndpointTracking = false
        endpointManuallyEdited = false
    }

    private func applyModelSuggestionIfNeeded() {
        guard state.mode == .add else { return }
        if modelManuallyEdited && !trimmedModel.isEmpty {
            return
        }
        let value = suggestedModel ?? ""
        model = value
        modelManuallyEdited = false
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(state.mode == .add ? "Add API key" : "Rotate API key")
                .font(.system(size: 16, weight: .semibold))

            if state.mode == .add {
                Picker("AI API", selection: $selectedProvider) {
                    ForEach(Self.popularProviderPresets) { preset in
                        Text(preset.company).tag(preset.id)
                    }
                    Text("Custom").tag(Self.customProviderOption)
                }
                .pickerStyle(.menu)

                if selectedProvider == Self.customProviderOption {
                    TextField("Custom provider slug (e.g. myprovider)", text: $customProvider)
                        .textFieldStyle(.roundedBorder)
                        .autocorrectionDisabled()
                } else {
                    Text("Provider: \(normalizedProvider)")
                        .font(.system(size: 11, weight: .medium, design: .monospaced))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }

                if !modelOptions.isEmpty {
                    Picker("Model", selection: $model) {
                        ForEach(modelOptions, id: \.self) { option in
                            Text(option).tag(option)
                        }
                    }
                    .pickerStyle(.menu)
                } else {
                    TextField("Model", text: $model)
                        .textFieldStyle(.roundedBorder)
                        .autocorrectionDisabled()
                }

                TextField("Endpoint URL (optional for known providers)", text: $endpoint)
                    .textFieldStyle(.roundedBorder)

                Text("Choose company + model, paste key, done. Endpoint is prefilled and can be overridden.")
                    .font(.system(size: 11, weight: .regular))
                    .foregroundStyle(TokfenceTheme.textSecondary)

                if !normalizedProvider.isEmpty && !isValidProviderName {
                    Text("Provider must use lowercase letters, numbers, '-' or '_'.")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(TokfenceTheme.danger)
                }

                if requiresEndpoint && trimmedEndpoint.isEmpty {
                    Text("Endpoint is required for unknown custom providers.")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(TokfenceTheme.warning)
                }

                if resolvedModel == nil {
                    Text("Model is required.")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(TokfenceTheme.warning)
                }
            } else {
                Text(TokfenceFormatting.providerLabel(state.provider))
                    .font(.system(size: 12, weight: .medium))
                    .foregroundStyle(TokfenceTheme.textSecondary)
                if !state.upstream.isEmpty {
                    Text(state.upstream)
                        .font(.system(size: 11, weight: .regular, design: .monospaced))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }
            }

            SecureField("Paste API key", text: $key)
                .textFieldStyle(.roundedBorder)

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .buttonStyle(.bordered)
                Button(state.mode == .add ? "Add" : "Rotate") {
                    guard canSubmit else { return }
                    onSubmit(VaultKeySubmission(
                        provider: state.mode == .add ? normalizedProvider : state.provider,
                        key: trimmedKey,
                        endpoint: state.mode == .add ? resolvedEndpoint : nil,
                        model: state.mode == .add ? resolvedModel : nil
                    ))
                    dismiss()
                }
                .buttonStyle(.borderedProminent)
                .tint(TokfenceTheme.accentPrimary)
                .disabled(!canSubmit)
            }
        }
        .padding(16)
        .frame(minWidth: 460)
        .onAppear {
            if state.mode == .add {
                if !state.provider.isEmpty,
                   Self.popularProviderPresets.contains(where: { $0.id == state.provider }) {
                    selectedProvider = state.provider
                } else {
                    selectedProvider = "anthropic"
                    customProvider = state.provider
                }
            }
            endpoint = state.upstream
            applyEndpointSuggestionIfNeeded()
            applyModelSuggestionIfNeeded()
        }
        .onChange(of: selectedProvider) { _, _ in
            applyEndpointSuggestionIfNeeded()
            applyModelSuggestionIfNeeded()
        }
        .onChange(of: customProvider) { _, _ in
            applyEndpointSuggestionIfNeeded()
            applyModelSuggestionIfNeeded()
        }
        .onChange(of: endpoint) { _, _ in
            if state.mode == .add && !suppressEndpointTracking {
                endpointManuallyEdited = true
            }
        }
        .onChange(of: model) { _, _ in
            if state.mode == .add {
                modelManuallyEdited = true
            }
        }
    }
}

private struct SetupWizardSheet: View {
    @ObservedObject var viewModel: TokfenceAppViewModel
    let knownUpstreams: [String: String]

    @Environment(\.dismiss) private var dismiss

    @State private var provider = "openai"
    @State private var endpoint = ""
    @State private var model = ""
    @State private var key = ""
    @State private var isRunning = false
    @State private var result: TokfenceSetupWizardResult?
    @State private var failureMessage = ""
    @State private var endpointEdited = false
    @State private var suppressEndpointTracking = false

    private static let endpointDefaults: [String: String] = [
        "anthropic": "https://api.anthropic.com",
        "openai": "https://api.openai.com",
        "google": "https://generativelanguage.googleapis.com",
        "mistral": "https://api.mistral.ai",
        "openrouter": "https://openrouter.ai/api",
        "groq": "https://api.groq.com/openai",
        "deepseek": "https://api.deepseek.com",
    ]

    private static let modelDefaults: [String: String] = [
        "anthropic": "claude-3-5-haiku-latest",
        "openai": "gpt-4o-mini",
        "mistral": "mistral-small-latest",
        "openrouter": "openai/gpt-4o-mini",
        "groq": "llama-3.1-8b-instant",
        "deepseek": "deepseek-chat",
    ]

    private var normalizedProvider: String {
        provider.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
    }

    private var trimmedEndpoint: String {
        endpoint.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedModel: String {
        model.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var trimmedKey: String {
        key.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    private var validProviderName: Bool {
        guard !normalizedProvider.isEmpty else { return false }
        return normalizedProvider.range(of: "^[a-z0-9][a-z0-9_-]{0,62}$", options: .regularExpression) != nil
    }

    private var suggestedEndpoint: String? {
        if let configured = knownUpstreams[normalizedProvider], !configured.isEmpty {
            return configured
        }
        return Self.endpointDefaults[normalizedProvider]
    }

    private var suggestedModel: String {
        if let preferred = viewModel.preferredModel(for: normalizedProvider) {
            return preferred
        }
        return Self.modelDefaults[normalizedProvider] ?? "gpt-4o-mini"
    }

    private var canRun: Bool {
        validProviderName && !trimmedKey.isEmpty && normalizedProvider != "google"
    }

    private func maybePrefillEndpoint() {
        if endpointEdited && !trimmedEndpoint.isEmpty {
            return
        }
        let suggested = suggestedEndpoint ?? ""
        suppressEndpointTracking = true
        endpoint = suggested
        suppressEndpointTracking = false
        endpointEdited = false
    }

    private func maybePrefillModel() {
        if trimmedModel.isEmpty {
            model = suggestedModel
        }
    }

    private func runWizard() {
        guard canRun else { return }
        isRunning = true
        failureMessage = ""
        result = nil

        Task {
            do {
                let output = try await viewModel.runSetupWizard(
                    provider: normalizedProvider,
                    endpoint: trimmedEndpoint.isEmpty ? nil : trimmedEndpoint,
                    key: trimmedKey,
                    model: trimmedModel
                )
                await MainActor.run {
                    result = output
                    isRunning = false
                }
            } catch {
                await MainActor.run {
                    failureMessage = error.localizedDescription
                    isRunning = false
                }
            }
        }
    }

    private func copyOpenClawLine() {
        guard let result else { return }
        let line = "base_url: \"\(result.baseURL)\""
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(line, forType: .string)
    }

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Setup Wizard")
                .font(.system(size: 18, weight: .semibold))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text("1-minute confidence flow: store key, run streaming request, verify logs and cost.")
                .font(.system(size: 12, weight: .regular))
                .foregroundStyle(TokfenceTheme.textSecondary)

            TextField("Provider name (e.g. openai, anthropic, deepseek)", text: $provider)
                .textFieldStyle(.roundedBorder)
                .autocorrectionDisabled()

            TextField("Endpoint URL (optional for known providers)", text: $endpoint)
                .textFieldStyle(.roundedBorder)

            TextField("Model for probe request", text: $model)
                .textFieldStyle(.roundedBorder)

            SecureField("API key", text: $key)
                .textFieldStyle(.roundedBorder)

            if normalizedProvider == "google" {
                Text("Automatic streaming probe for google is not supported yet. Use another provider for the wizard.")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(TokfenceTheme.warning)
            }

            if !failureMessage.isEmpty {
                Text(failureMessage)
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(TokfenceTheme.danger)
            }

            if let result {
                TokfenceCard {
                    stepRow("Daemon reachable", ok: result.daemonReachable)
                    stepRow("Key stored in vault", ok: result.keyStored)
                    stepRow(
                        "Streaming first chunk",
                        ok: result.probe.streamChunkReceived,
                        detail: result.probe.firstChunkMS.map { "\($0) ms" } ?? "not observed"
                    )
                    stepRow(
                        "HTTP status",
                        ok: (200..<300).contains(result.probe.statusCode),
                        detail: "\(result.probe.statusCode)"
                    )
                    stepRow("Log entry created", ok: result.logFound, detail: result.logRecord?.id ?? "missing")
                    stepRow("Tokens logged", ok: result.tokensLogged)
                    stepRow("Cost logged", ok: result.costLogged)

                    if !result.probe.responsePreview.isEmpty && !result.probe.streamChunkReceived {
                        Text("Probe response preview")
                            .font(.system(size: 11, weight: .semibold))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                        Text(result.probe.responsePreview)
                            .font(.system(size: 11, weight: .regular, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                            .lineLimit(4)
                    }

                    HStack {
                        Text("OpenClaw config:")
                            .font(.system(size: 11, weight: .medium))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                        Text("base_url: \"\(result.baseURL)\"")
                            .font(.system(size: 11, weight: .medium, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textPrimary)
                            .textSelection(.enabled)
                        Spacer()
                        Button("Copy") {
                            copyOpenClawLine()
                        }
                        .buttonStyle(.bordered)
                    }
                }
            }

            HStack {
                if isRunning {
                    ProgressView()
                        .controlSize(.small)
                    Text("Running end-to-end verification...")
                        .font(.system(size: 11, weight: .medium))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }
                Spacer()
                Button("Close") {
                    dismiss()
                }
                .buttonStyle(.bordered)
                Button("Run Verification") {
                    runWizard()
                }
                .buttonStyle(.borderedProminent)
                .tint(TokfenceTheme.accentPrimary)
                .disabled(isRunning || !canRun)
            }
        }
        .padding(16)
        .frame(minWidth: 640, minHeight: 560)
        .onAppear {
            maybePrefillEndpoint()
            maybePrefillModel()
        }
        .onChange(of: provider) { _, _ in
            maybePrefillEndpoint()
            maybePrefillModel()
        }
        .onChange(of: endpoint) { _, _ in
            if !suppressEndpointTracking {
                endpointEdited = true
            }
        }
    }

    private func stepRow(_ title: String, ok: Bool, detail: String? = nil) -> some View {
        HStack(spacing: 8) {
            Image(systemName: ok ? "checkmark.circle.fill" : "xmark.circle.fill")
                .foregroundStyle(ok ? TokfenceTheme.healthy : TokfenceTheme.warning)
                .font(.system(size: 12, weight: .medium))
            Text(title)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.textPrimary)
            if let detail, !detail.isEmpty {
                Text(detail)
                    .font(.system(size: 11, weight: .regular, design: .monospaced))
                    .foregroundStyle(TokfenceTheme.textSecondary)
            }
            Spacer()
        }
    }
}

private struct BudgetEditorState: Identifiable {
    let id = UUID()
    let provider: String
    let period: String
    let amount: String
}

private struct BudgetEditorSheet: View {
    let state: BudgetEditorState
    let onSubmit: (String, String) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var amount = ""
    @State private var period = "daily"

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Edit Budget")
                .font(.system(size: 16, weight: .semibold))
            Text(TokfenceFormatting.providerLabel(state.provider))
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
            TextField("Amount in USD", text: $amount)
                .textFieldStyle(.roundedBorder)
            Picker("Period", selection: $period) {
                Text("daily").tag("daily")
                Text("monthly").tag("monthly")
            }
            .pickerStyle(.segmented)

            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .buttonStyle(.bordered)
                Button("Save") {
                    onSubmit(amount, period)
                    dismiss()
                }
                .buttonStyle(.borderedProminent)
                .tint(TokfenceTheme.accentPrimary)
            }
        }
        .padding(16)
        .frame(minWidth: 360)
        .onAppear {
            amount = state.amount
            period = state.period.lowercased()
        }
    }
}

private struct RateLimitEditorState: Identifiable {
    let id = UUID()
    let provider: String
    let rpm: String
}

private struct RateLimitEditorSheet: View {
    let state: RateLimitEditorState
    let onSubmit: (String) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var rpm = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("Rate Limit")
                .font(.system(size: 16, weight: .semibold))
            Text(TokfenceFormatting.providerLabel(state.provider))
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
            TextField("Requests per minute", text: $rpm)
                .textFieldStyle(.roundedBorder)
            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .buttonStyle(.bordered)
                Button("Save") {
                    onSubmit(rpm)
                    dismiss()
                }
                .buttonStyle(.borderedProminent)
                .tint(TokfenceTheme.accentPrimary)
            }
        }
        .padding(16)
        .frame(minWidth: 320)
        .onAppear {
            rpm = state.rpm
        }
    }
}
