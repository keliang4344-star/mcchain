import SwiftUI

@main
struct McMinerApp: App {

    @StateObject private var model: AppModel

    init() {
        let model = AppModel()
        _model = StateObject(wrappedValue: model)
        BackgroundHeartbeat.register {
            await model.refresh()
            await model.heartbeat()
        }
    }

    var body: some Scene {
        WindowGroup {
            RootView()
                .environmentObject(model)
                .onAppear { BackgroundHeartbeat.schedule() }
        }
    }
}
