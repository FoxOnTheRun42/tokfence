import Foundation

struct TokfenceCommandRunner {
    private let binaryPathKey = "tokfence.desktop.binaryPath"

    var configuredBinaryPath: String {
        get {
            if let stored = UserDefaults.standard.string(forKey: binaryPathKey), !stored.isEmpty {
                return stored
            }
            return resolveDefaultBinaryPath()
        }
        set {
            UserDefaults.standard.set(newValue, forKey: binaryPathKey)
        }
    }

    func fetchSnapshot() throws -> TokfenceSnapshot {
        let output = try run(arguments: ["widget", "render", "--json"])
        guard let data = output.data(using: .utf8) else {
            throw NSError(domain: "TokfenceCommandRunner", code: 1, userInfo: [NSLocalizedDescriptionKey: "Unable to parse command output as UTF-8"])
        }
        return try JSONDecoder.tokfence.decode(TokfenceSnapshot.self, from: data)
    }

    func runAction(arguments: [String]) throws {
        _ = try run(arguments: arguments)
    }

    private func run(arguments: [String]) throws -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: configuredBinaryPath)
        process.arguments = arguments

        let outputPipe = Pipe()
        let errorPipe = Pipe()
        process.standardOutput = outputPipe
        process.standardError = errorPipe

        do {
            try process.run()
            process.waitUntilExit()
        } catch {
            throw NSError(domain: "TokfenceCommandRunner", code: 2, userInfo: [
                NSLocalizedDescriptionKey: "Failed to start tokfence binary at \(configuredBinaryPath). Set a valid binary path in the app."
            ])
        }

        let outputData = outputPipe.fileHandleForReading.readDataToEndOfFile()
        let errorData = errorPipe.fileHandleForReading.readDataToEndOfFile()
        let output = String(decoding: outputData, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        let stderr = String(decoding: errorData, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)

        if process.terminationStatus != 0 {
            let message = stderr.isEmpty ? "tokfence command failed (exit \(process.terminationStatus))" : stderr
            throw NSError(domain: "TokfenceCommandRunner", code: Int(process.terminationStatus), userInfo: [NSLocalizedDescriptionKey: message])
        }

        return output
    }

    private func resolveDefaultBinaryPath() -> String {
        let preferred = [
            "\\(FileManager.default.homeDirectoryForCurrentUser.path)/tmp/glasbox/glasbox/bin/tokfence",
            "/opt/homebrew/bin/tokfence",
            "/usr/local/bin/tokfence"
        ]
        for path in preferred where FileManager.default.isExecutableFile(atPath: path) {
            return path
        }

        let whichProcess = Process()
        whichProcess.executableURL = URL(fileURLWithPath: "/usr/bin/which")
        whichProcess.arguments = ["tokfence"]
        let pipe = Pipe()
        whichProcess.standardOutput = pipe
        try? whichProcess.run()
        whichProcess.waitUntilExit()
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        let detected = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
        if !detected.isEmpty {
            return detected
        }
        return "/opt/homebrew/bin/tokfence"
    }
}
