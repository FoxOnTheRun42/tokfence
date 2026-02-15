import Foundation

enum TokfenceSharedStore {
    static func snapshotFileURL() -> URL {
        let home = FileManager.default.homeDirectoryForCurrentUser
        return home.appendingPathComponent(".tokfence/desktop_snapshot.json")
    }

    static func loadSnapshot() -> TokfenceSnapshot {
        let url = snapshotFileURL()
        guard let data = try? Data(contentsOf: url) else {
            return .empty
        }
        guard let snapshot = try? JSONDecoder.tokfence.decode(TokfenceSnapshot.self, from: data) else {
            return .empty
        }
        return snapshot
    }

    static func saveSnapshot(_ snapshot: TokfenceSnapshot) {
        let url = snapshotFileURL()
        do {
            try FileManager.default.createDirectory(at: url.deletingLastPathComponent(), withIntermediateDirectories: true)
            let data = try JSONEncoder.tokfence.encode(snapshot)
            try data.write(to: url, options: .atomic)
        } catch {
            NSLog("TokfenceSharedStore save error: \(error.localizedDescription)")
        }
    }
}
