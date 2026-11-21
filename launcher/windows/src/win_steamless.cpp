#include "win_steamless.h"

#include "win_native.h"

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QProcess>

namespace sapie::win {
namespace {

SteamlessOutcome outcome(SteamlessResult result, const QString& message,
                         const QString& executable = {})
{
    SteamlessOutcome value;
    value.result = result;
    value.message = message;
    value.unpackedExecutable = executable;
    return value;
}

QString sidecarPath()
{
    const QString besideApp = QDir(QCoreApplication::applicationDirPath())
        .filePath(QStringLiteral("steamless/Steamless.CLI.exe"));
    if (QFileInfo::exists(besideApp)) {
        return QFileInfo(besideApp).canonicalFilePath();
    }
    const QString inData = QDir(dataDirectory())
        .filePath(QStringLiteral("steamless/Steamless.CLI.exe"));
    return QFileInfo::exists(inData) ? QFileInfo(inData).canonicalFilePath() : QString();
}

}

SteamlessOutcome runSteamless(const QString& gameExecutable, int timeoutMs)
{
    const QString sidecar = sidecarPath();
    if (sidecar.isEmpty()) {
        return outcome(SteamlessResult::Error,
                       QStringLiteral("Steamless.CLI.exe is not installed with the launcher."));
    }

    const QFileInfo executable(gameExecutable);
    if (!executable.exists() || !executable.isFile()) {
        return outcome(SteamlessResult::Error,
                       QStringLiteral("The game executable does not exist."));
    }
    if (executable.suffix().compare(QStringLiteral("exe"), Qt::CaseInsensitive) != 0) {
        return outcome(SteamlessResult::Error,
                       QStringLiteral("Steamless accepts an executable ending in .exe."));
    }
    const QString canonical = executable.canonicalFilePath();
    const QString sidecarDirectory = QFileInfo(sidecar).canonicalPath();
    if (canonical.startsWith(sidecarDirectory, Qt::CaseInsensitive)) {
        return outcome(SteamlessResult::Error,
                       QStringLiteral("Refusing to unpack an executable inside the Steamless folder."));
    }

    QProcess process;
    process.setProgram(sidecar);
    process.setArguments({canonical});
    process.setWorkingDirectory(sidecarDirectory);
    process.start();
    if (!process.waitForStarted(5000)) {
        return outcome(SteamlessResult::Error,
                       QStringLiteral("Steamless could not be started."));
    }
    if (!process.waitForFinished(timeoutMs)) {
        process.kill();
        process.waitForFinished(5000);
        return outcome(SteamlessResult::Error,
                       QStringLiteral("Steamless did not finish in time."));
    }

    const QByteArray output = (process.readAllStandardOutput() + process.readAllStandardError()).left(65536);
    const QString text = QString::fromLocal8Bit(output);

    if (text.contains(QStringLiteral("Successfully unpacked file!"))) {
        const QString unpacked = executable.absoluteDir()
            .filePath(executable.completeBaseName() + QStringLiteral(".unpacked.exe"));
        if (QFileInfo::exists(unpacked)) {
            return outcome(SteamlessResult::Unpacked, text.trimmed(), unpacked);
        }
        return outcome(SteamlessResult::Error,
                       QStringLiteral("Steamless reported success but wrote no unpacked executable."));
    }

    if (text.contains(QStringLiteral("Failed to unpack file."))
        || text.contains(QStringLiteral("All unpackers failed"))
        || text.contains(QStringLiteral("Invalid input file given"))) {
        return outcome(SteamlessResult::NotPacked, text.trimmed());
    }

    return outcome(SteamlessResult::Error,
                   QStringLiteral("Steamless gave no clear result. %1").arg(text.trimmed()));
}

}
