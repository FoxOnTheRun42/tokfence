import SwiftUI

@main
struct TokfenceDesktopApp: App {
    @StateObject private var viewModel = DashboardViewModel()

    var body: some Scene {
        WindowGroup {
            ContentView(viewModel: viewModel)
                .frame(minWidth: 980, minHeight: 700)
                .task { viewModel.start() }
                .onDisappear { viewModel.stop() }
        }
        .windowResizability(.contentMinSize)
    }
}
