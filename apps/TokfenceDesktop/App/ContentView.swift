import AppKit
import Charts
import SwiftUI

struct ContentView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    var body: some View {
        HStack(spacing: 0) {
            sidebar
                .frame(width: 220)
                .background(TokfenceTheme.bgSecondary)
                .animation(TokfenceTheme.uiAnimation, value: viewModel.selectedSection)

            Divider()

            mainContent
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .animation(TokfenceTheme.uiAnimation, value: viewModel.selectedSection)
                .background(TokfenceTheme.bgPrimary)
        }
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
                .frame(maxWidth: 300)
                .transition(.move(edge: .top).combined(with: .opacity))
                .zIndex(1)
            }
        }
        .animation(TokfenceTheme.uiAnimation, value: viewModel.showErrorToast)
        .background(TokfenceTheme.bgPrimary)
    }

    private var sidebar: some View {
        VStack(alignment: .leading, spacing: 12) {
            HStack(spacing: 8) {
                RoundedRectangle(cornerRadius: 6, style: .continuous)
                    .fill(TokfenceTheme.accentPrimary)
                    .frame(width: 24, height: 24)
                    .overlay {
                        Image(systemName: "lock.fill")
                            .font(.system(size: 12, weight: .bold))
                            .foregroundStyle(.white)
                    }
                Text("Tokfence")
                    .font(.system(size: 14, weight: .semibold))
                    .foregroundStyle(TokfenceTheme.textPrimary)
                Spacer(minLength: 0)
                Menu {
                    Button("Refresh") { Task { await viewModel.refreshAll() } }
                    Divider()
                    Button("Start Daemon") { Task { await viewModel.startDaemon() } }
                    Button("Stop Daemon") { Task { await viewModel.stopDaemon() } }
                } label: {
                    Circle()
                        .fill(viewModel.snapshot.killSwitchActive ? TokfenceTheme.danger : (viewModel.snapshot.running ? TokfenceTheme.healthy : TokfenceTheme.danger))
                        .frame(width: 10, height: 10)
                }
                .menuStyle(.borderlessButton)
                .help("Daemon controls")
            }
            .padding(.top, 6)

                ForEach(TokfenceSection.allCases) { section in
                    Button {
                        withAnimation(TokfenceTheme.uiAnimation) {
                            viewModel.selectedSection = section
                        }
                    } label: {
                        HStack(spacing: 8) {
                            Image(systemName: section.symbol)
                                .font(.system(size: 13, weight: .medium))
                            .frame(width: 16)
                        Text(section.title)
                            .font(.system(size: 13, weight: viewModel.selectedSection == section ? .semibold : .medium))
                        Spacer(minLength: 0)
                    }
                    .padding(.vertical, 8)
                    .padding(.horizontal, 10)
                    .foregroundStyle(viewModel.selectedSection == section ? TokfenceTheme.accentPrimary : TokfenceTheme.textPrimary)
                    .background(
                        RoundedRectangle(cornerRadius: TokfenceTheme.badgeCorner, style: .continuous)
                            .fill(viewModel.selectedSection == section ? TokfenceTheme.accentMuted : Color.clear)
                    )
                }
                .buttonStyle(.plain)
            }

            Spacer(minLength: 12)

            if viewModel.snapshot.killSwitchActive {
                Text("KILLED")
                    .font(.system(size: 11, weight: .semibold))
                    .foregroundStyle(.white)
                    .padding(.vertical, 4)
                    .padding(.horizontal, 8)
                    .background(TokfenceTheme.danger, in: RoundedRectangle(cornerRadius: 4, style: .continuous))
            }

            TokfenceCard {
                Text("Today")
                    .font(.system(size: 11, weight: .medium))
                    .foregroundStyle(TokfenceTheme.textSecondary)
                if let budget = viewModel.globalDailyBudget {
                    Text("\(TokfenceFormatting.usd(cents: budget.currentSpendCents)) / \(TokfenceFormatting.usd(cents: budget.limitCents))")
                        .font(.system(size: 12, weight: .semibold, design: .monospaced))
                        .foregroundStyle(TokfenceTheme.textPrimary)
                    TokfenceBudgetProgressBar(current: budget.currentSpendCents, limit: budget.limitCents)
                } else {
                    Text(TokfenceFormatting.usd(cents: viewModel.snapshot.todayCostCents))
                        .font(.system(size: 12, weight: .semibold, design: .monospaced))
                        .foregroundStyle(TokfenceTheme.textPrimary)
                    Text("No daily budget configured")
                        .font(.system(size: 11, weight: .regular))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }
            }
        }
        .padding(.vertical, 16)
        .padding(.horizontal, 12)
    }

    @ViewBuilder
    private var mainContent: some View {
        ZStack {
            switch viewModel.selectedSection {
            case .dashboard:
                DashboardSectionView(viewModel: viewModel)
            case .vault:
                VaultSectionView(viewModel: viewModel)
            case .logs:
                LogsSectionView(viewModel: viewModel)
            case .budget:
                BudgetSectionView(viewModel: viewModel)
            case .providers:
                ProvidersSectionView(viewModel: viewModel)
            case .settings:
                SettingsSectionView(viewModel: viewModel)
            }
        }
        .id(viewModel.selectedSection)
    }
}

private struct DashboardSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel
    @State private var selectedRequestID: String?

    private struct ChartPoint: Identifiable {
        let id = UUID()
        let hour: Date
        let provider: String
        let count: Int
    }

    private var chartPoints: [ChartPoint] {
        var points: [ChartPoint] = []
        for bucket in viewModel.hourlyBuckets {
            for provider in viewModel.providers {
                let count = bucket.counts[provider, default: 0]
                if count > 0 {
                    points.append(ChartPoint(hour: bucket.hourStart, provider: provider, count: count))
                }
            }
        }
        return points
    }

    private var medianLatencyMS: Int {
        let values = viewModel.logs.map(\.latencyMS).sorted()
        guard !values.isEmpty else { return 0 }
        return values[values.count / 2]
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                TokfenceSectionHeader(
                    title: "Dashboard",
                    subtitle: "What happened today"
                , trailing: AnyView(
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
                ))

                HStack(spacing: 12) {
                    metricCard(title: "Requests", value: "\(viewModel.snapshot.todayRequests)", subtitle: "today", icon: "arrow.up.arrow.down")
                    metricCard(title: "Tokens", value: "\(TokfenceFormatting.tokens(viewModel.snapshot.todayInputTokens + viewModel.snapshot.todayOutputTokens))", subtitle: "in + out", icon: "textformat.123")
                    metricCard(title: "Cost", value: TokfenceFormatting.usd(cents: viewModel.snapshot.todayCostCents), subtitle: "estimated today", icon: "dollarsign.circle")
                    metricCard(title: "Latency", value: TokfenceFormatting.latency(ms: medianLatencyMS), subtitle: "median p50", icon: "clock")
                }

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
                        Chart(chartPoints) { point in
                            BarMark(
                                x: .value("Hour", point.hour, unit: .hour),
                                y: .value("Requests", point.count)
                            )
                            .foregroundStyle(by: .value("Provider", TokfenceFormatting.providerLabel(point.provider)))
                        }
                        .frame(height: 180)
                        .chartXAxis {
                            AxisMarks(values: .stride(by: .hour, count: 4)) { _ in
                                AxisGridLine()
                                AxisTick()
                                AxisValueLabel(format: .dateTime.hour())
                            }
                        }
                        .chartLegend(position: .bottom, spacing: 10)
                    }
                }

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
                        Table(viewModel.logs.prefix(20), selection: $selectedRequestID) {
                            TableColumn("Time") { record in
                                Text(TokfenceFormatting.timeOfDay(record.timestamp))
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                                    .foregroundStyle(TokfenceTheme.textSecondary)
                            }
                            .width(76)

                            TableColumn("Provider") { record in
                                TokfenceProviderBadge(provider: record.provider, active: true)
                            }
                            .width(90)

                            TableColumn("Model") { record in
                                Text(record.model.isEmpty ? "(unknown)" : record.model)
                                    .font(.system(size: 12, weight: .regular, design: .monospaced))
                                    .lineLimit(1)
                            }
                            .width(min: 120, ideal: 180)

                            TableColumn("Tokens") { record in
                                Text("\(TokfenceFormatting.tokens(record.inputTokens)) -> \(TokfenceFormatting.tokens(record.outputTokens))")
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                            }
                            .width(110)

                            TableColumn("Cost") { record in
                                Text(TokfenceFormatting.usd(cents: record.estimatedCostCents))
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                            }
                            .width(70)

                            TableColumn("Latency") { record in
                                Text(TokfenceFormatting.latency(ms: record.latencyMS))
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                            }
                            .width(70)

                            TableColumn("Status") { record in
                                Circle()
                                    .fill(TokfenceTheme.statusColor(for: record.statusCode))
                                    .frame(width: 8, height: 8)
                            }
                            .width(50)
                        }
                        .frame(minHeight: 240)
                        .onChange(of: selectedRequestID) { _, newValue in
                            guard let newValue else { return }
                            selectedRequestID = newValue
                        }
                    }
                }
            }
            .padding(16)
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

    private func metricCard(title: String, value: String, subtitle: String, icon: String) -> some View {
        TokfenceCard {
            Image(systemName: icon)
                .font(.system(size: 15, weight: .medium))
                .foregroundStyle(TokfenceTheme.accentPrimary)
            Text(value)
                .font(.system(size: 20, weight: .semibold, design: .monospaced))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text(title)
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.textPrimary)
            Text(subtitle)
                .font(.system(size: 11, weight: .regular))
                .foregroundStyle(TokfenceTheme.textSecondary)
        }
    }

}

private struct VaultSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    @State private var editor: VaultKeyEditorState?
    @State private var providerToRemove: String?

    private var providersToShow: [String] {
        if viewModel.providers.isEmpty {
            return ["anthropic", "openai", "google", "mistral", "groq", "openrouter"]
        }
        return viewModel.providers
    }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                TokfenceSectionHeader(
                    title: "Vault",
                    subtitle: "Manage API keys securely",
                    trailing: AnyView(
                        Button {
                            if let provider = providersToShow.first(where: { !viewModel.snapshot.vaultProviders.contains($0) }) {
                                editor = VaultKeyEditorState(provider: provider, mode: .add)
                            }
                        } label: {
                            Label("Add Key", systemImage: "plus")
                        }
                        .buttonStyle(.borderedProminent)
                        .tint(TokfenceTheme.accentPrimary)
                    )
                )

                ForEach(providersToShow, id: \.self) { provider in
                    let hasKey = viewModel.snapshot.vaultProviders.contains(provider)
                    let isRevoked = viewModel.snapshot.revokedProviders.contains(provider)
                    let lastUsed = viewModel.logs.first(where: { $0.provider == provider })?.timestamp

                    TokfenceCard {
                        HStack(alignment: .center) {
                            Circle()
                                .fill(hasKey ? (isRevoked ? TokfenceTheme.warning : TokfenceTheme.healthy) : TokfenceTheme.textTertiary)
                                .frame(width: 10, height: 10)
                            Text(TokfenceFormatting.providerLabel(provider))
                                .font(.system(size: 16, weight: .semibold))
                                .foregroundStyle(TokfenceTheme.textPrimary)
                            Spacer()
                            if hasKey {
                                Button("Rotate") {
                                    editor = VaultKeyEditorState(provider: provider, mode: .rotate)
                                }
                                .buttonStyle(.bordered)

                                Button("Remove", role: .destructive) {
                                    providerToRemove = provider
                                }
                                .buttonStyle(.bordered)
                            } else {
                                Button("Add Key") {
                                    editor = VaultKeyEditorState(provider: provider, mode: .add)
                                }
                                .buttonStyle(.borderedProminent)
                                .tint(TokfenceTheme.accentPrimary)
                            }
                        }

                        Text(hasKey ? "Key: configured (masked)" : "Key: not configured")
                            .font(.system(size: 12, weight: .medium, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textPrimary)

                        Text(
                            hasKey
                                ? "Last used: " + (lastUsed.map { TokfenceFormatting.relative($0) } ?? "never")
                                : "Agents using this provider will fail until a key is added."
                        )
                            .font(.system(size: 11, weight: .regular))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                    }
                }

                TokfenceCard {
                    Text("Vault backend: macOS Keychain (default)")
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
            .padding(16)
        }
        .sheet(item: $editor) { state in
            VaultKeyEditorSheet(state: state) { key in
                Task {
                    if state.mode == .add {
                        await viewModel.addVaultKey(provider: state.provider, key: key)
                    } else {
                        await viewModel.rotateVaultKey(provider: state.provider, key: key)
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

private struct LogsSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel
    @State private var selectedRequestID: String?

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            TokfenceSectionHeader(
                title: "Logs",
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
                        Table(viewModel.filteredLogs, selection: $selectedRequestID) {
                            TableColumn("Time") { record in
                                Text(TokfenceFormatting.timeOfDay(record.timestamp))
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                                    .foregroundStyle(TokfenceTheme.textSecondary)
                            }
                            .width(80)

                            TableColumn("Provider") { record in
                                TokfenceProviderBadge(provider: record.provider, active: true)
                            }
                            .width(95)

                            TableColumn("Model") { record in
                                Text(record.model)
                                    .font(.system(size: 11, weight: .regular, design: .monospaced))
                                    .lineLimit(1)
                            }
                            .width(min: 140, ideal: 210)

                            TableColumn("Tokens") { record in
                                Text("\(TokfenceFormatting.tokens(record.inputTokens)) -> \(TokfenceFormatting.tokens(record.outputTokens))")
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                            }
                            .width(120)

                            TableColumn("Cost") { record in
                                Text(TokfenceFormatting.usd(cents: record.estimatedCostCents))
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                            }
                            .width(70)

                            TableColumn("Latency") { record in
                                Text(TokfenceFormatting.latency(ms: record.latencyMS))
                                    .font(.system(size: 11, weight: .medium, design: .monospaced))
                            }
                            .width(80)

                            TableColumn("Status") { record in
                                Circle()
                                    .fill(TokfenceTheme.statusColor(for: record.statusCode))
                                    .frame(width: 8, height: 8)
                            }
                            .width(50)
                        }
                        .frame(minHeight: 420)
                    }
                }

                if let selectedRequestID, let record = viewModel.logs.first(where: { $0.id == selectedRequestID }) {
                    TokfenceCard {
                        LogDetailPanel(record: record)
                    }
                    .frame(minWidth: 320, idealWidth: 360)
                }
            }
        }
        .padding(16)
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
                .frame(width: 160)

                Picker("Status", selection: $viewModel.logStatusFilter) {
                    ForEach(TokfenceLogStatusFilter.allCases) { filter in
                        Text(filter.title).tag(filter)
                    }
                }
                .labelsHidden()
                .frame(width: 180)

                Picker("Range", selection: $viewModel.logTimeRange) {
                    Text("1h").tag(TokfenceTimeRange.oneHour)
                    Text("6h").tag(TokfenceTimeRange.sixHours)
                    Text("24h").tag(TokfenceTimeRange.twentyFourHours)
                    Text("7d").tag(TokfenceTimeRange.sevenDays)
                }
                .pickerStyle(.segmented)
                .frame(width: 230)

                TextField("Search model, request ID, caller", text: $viewModel.logQuery)
                    .textFieldStyle(.roundedBorder)
            }
        }
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
            }
            .padding(16)
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

                ForEach(viewModel.providerOverview) { provider in
                    TokfenceCard {
                        HStack {
                            Circle()
                                .fill(provider.isRevoked ? TokfenceTheme.warning : TokfenceTheme.healthy)
                                .frame(width: 10, height: 10)
                            Text(TokfenceFormatting.providerLabel(provider.provider))
                                .font(.system(size: 15, weight: .semibold))
                            Spacer()
                            if provider.isRevoked {
                                Button("Restore") {
                                    providerToRestore = provider.provider
                                }
                                .buttonStyle(.borderedProminent)
                                .tint(TokfenceTheme.healthy)
                            } else {
                                Button("Revoke") {
                                    providerToRevoke = provider.provider
                                }
                                .buttonStyle(.bordered)
                            }
                        }

                        Text("Endpoint: \(provider.upstream)")
                            .font(.system(size: 11, weight: .regular, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textSecondary)

                        Text("Key: \(provider.hasKey ? "configured" : "missing")")
                            .font(.system(size: 12, weight: .medium))

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
                    .opacity(provider.isRevoked ? 0.72 : 1)
                }
            }
            .padding(16)
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

private struct SettingsSectionView: View {
    @ObservedObject var viewModel: TokfenceAppViewModel

    @State private var shellSnippet = ""

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                TokfenceSectionHeader(title: "Settings", subtitle: "Daemon, paths and shell integration")

                TokfenceCard {
                    Text("Daemon")
                        .font(.system(size: 15, weight: .semibold))
                    HStack(spacing: 12) {
                        TokfenceStatusDot(
                            color: viewModel.snapshot.running ? TokfenceTheme.healthy : TokfenceTheme.danger,
                            label: viewModel.snapshot.running ? "Online" : "Offline"
                        )
                        Text(viewModel.snapshot.addr ?? "127.0.0.1:9471")
                            .font(.system(size: 12, weight: .regular, design: .monospaced))
                            .foregroundStyle(TokfenceTheme.textSecondary)
                    }
                    HStack(spacing: 8) {
                        Button("Start") { Task { await viewModel.startDaemon() } }
                            .buttonStyle(.bordered)
                        Button("Stop") { Task { await viewModel.stopDaemon() } }
                            .buttonStyle(.bordered)
                        Button("Refresh") { Task { await viewModel.refreshAll() } }
                            .buttonStyle(.bordered)
                    }
                }

                TokfenceCard {
                    Text("Desktop Integration")
                        .font(.system(size: 15, weight: .semibold))
                    TextField("Tokfence binary path", text: $viewModel.binaryPath)
                        .textFieldStyle(.roundedBorder)
                    HStack(spacing: 8) {
                        Button("Save") { viewModel.saveBinaryPath() }
                            .buttonStyle(.borderedProminent)
                            .tint(TokfenceTheme.accentPrimary)
                        Button("Open Data Folder") { viewModel.openDataFolder() }
                            .buttonStyle(.bordered)
                    }
                }

                TokfenceCard {
                    Text("Shell")
                        .font(.system(size: 15, weight: .semibold))
                    TextEditor(text: Binding(
                        get: { shellSnippet.isEmpty ? viewModel.shellSnippet() : shellSnippet },
                        set: { shellSnippet = $0 }
                    ))
                    .font(.system(size: 12, weight: .medium, design: .monospaced))
                    .frame(height: 72)
                    .scrollContentBackground(.hidden)
                    .background(TokfenceTheme.bgTertiary, in: RoundedRectangle(cornerRadius: 6, style: .continuous))
                    HStack(spacing: 8) {
                        Button("Copy") {
                            let value = shellSnippet.isEmpty ? viewModel.shellSnippet() : shellSnippet
                            NSPasteboard.general.clearContents()
                            NSPasteboard.general.setString(value, forType: .string)
                        }
                        .buttonStyle(.bordered)
                    }
                }

                TokfenceCard {
                    Text("About")
                        .font(.system(size: 15, weight: .semibold))
                    Text("Tokfence Desktop")
                        .font(.system(size: 12, weight: .medium))
                    Text("Version \(Bundle.main.infoDictionary?["CFBundleShortVersionString"] as? String ?? "0.1.0")")
                        .font(.system(size: 11, weight: .regular))
                        .foregroundStyle(TokfenceTheme.textSecondary)
                }
            }
            .padding(16)
        }
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

private struct VaultKeyEditorState: Identifiable {
    enum Mode {
        case add
        case rotate
    }

    let id = UUID()
    let provider: String
    let mode: Mode
}

private struct VaultKeyEditorSheet: View {
    let state: VaultKeyEditorState
    let onSubmit: (String) -> Void

    @Environment(\.dismiss) private var dismiss
    @State private var key = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text(state.mode == .add ? "Add API key" : "Rotate API key")
                .font(.system(size: 16, weight: .semibold))
            Text(TokfenceFormatting.providerLabel(state.provider))
                .font(.system(size: 12, weight: .medium))
                .foregroundStyle(TokfenceTheme.textSecondary)
            SecureField("Paste API key", text: $key)
                .textFieldStyle(.roundedBorder)
            HStack {
                Spacer()
                Button("Cancel") { dismiss() }
                    .buttonStyle(.bordered)
                Button(state.mode == .add ? "Add" : "Rotate") {
                    let trimmed = key.trimmingCharacters(in: .whitespacesAndNewlines)
                    guard !trimmed.isEmpty else { return }
                    onSubmit(trimmed)
                    dismiss()
                }
                .buttonStyle(.borderedProminent)
                .tint(TokfenceTheme.accentPrimary)
                .disabled(key.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
            }
        }
        .padding(16)
        .frame(minWidth: 420)
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
