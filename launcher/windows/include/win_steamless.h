#pragma once

#include <QString>

namespace sapie::win {

enum class SteamlessResult {
    Unpacked,
    NotPacked,
    Error
};

struct SteamlessOutcome {
    SteamlessResult result = SteamlessResult::Error;
    QString unpackedExecutable;
    QString message;
};

SteamlessOutcome runSteamless(const QString& gameExecutable, int timeoutMs = 60000);

}
