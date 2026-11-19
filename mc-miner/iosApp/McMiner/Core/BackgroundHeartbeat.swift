import BackgroundTasks
import Foundation

/// 后台心跳（BGAppRefreshTask）。
///
/// iOS 的现实约束：后台刷新由系统调度，不保证 120s 周期，也可能整天不触发。
/// 因此这里的定位是「补一次心跳」，真正的在线时长由前台定时器 + 每次开屏补发保证；
/// 链上离线宽限（100 块 ≈ 400s）远小于 iOS 后台调度间隔，长期熄屏挂机会掉线，
/// 产品侧需在文案中明确「需保持前台/充电挂机」。
enum BackgroundHeartbeat {

    static let identifier = "com.mcchain.miner.heartbeat"

    static func register(handler: @escaping () async -> Void) {
        BGTaskScheduler.shared.register(forTaskWithIdentifier: identifier, using: nil) { task in
            guard let refresh = task as? BGAppRefreshTask else {
                task.setTaskCompleted(success: false)
                return
            }
            schedule()
            let work = Task { await handler(); refresh.setTaskCompleted(success: true) }
            refresh.expirationHandler = { work.cancel() }
        }
    }

    static func schedule() {
        let request = BGAppRefreshTaskRequest(identifier: identifier)
        request.earliestBeginDate = Date(timeIntervalSinceNow: 15 * 60)
        try? BGTaskScheduler.shared.submit(request)
    }
}
