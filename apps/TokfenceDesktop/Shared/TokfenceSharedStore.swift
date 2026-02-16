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
        let folder = url.deletingLastPathComponent()
        do {
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: folder.path)
            let data = try JSONEncoder.tokfence.encode(snapshot)
            let tmp = url.appendingPathExtension("tmp")
            defer {
                _ = try? FileManager.default.removeItem(at: tmp)
            }
            try data.write(to: tmp)
            try FileManager.default.setAttributes([.posixPermissions: 0o600], ofItemAtPath: tmp.path)
            if FileManager.default.fileExists(atPath: url.path) {
                try FileManager.default.removeItem(at: url)
            }
            try FileManager.default.moveItem(at: tmp, to: url)
        } catch {
            NSLog("TokfenceSharedStore save error: \(error.localizedDescription)")
        }
    }
}
