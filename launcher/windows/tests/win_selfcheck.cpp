#include "win_acl.h"
#include "win_native.h"

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QTemporaryDir>
#include <QTextStream>

#include <cstdio>

namespace {

int check(bool condition, const char* message)
{
    if (!condition) {
        QTextStream(stderr) << "FAIL: " << message << Qt::endl;
        return 1;
    }
    return 0;
}

}

int main(int argc, char** argv)
{
    QCoreApplication app(argc, argv);
    int result = 0;

    QString lobby;
    QString error;
    result |= check(parseProtocolUrl(QStringLiteral("sapie://join/123?exp=1&sig=abc"),
                                     &lobby, &error),
                    "valid invite link is accepted");
    result |= check(lobby == QStringLiteral("123"), "invite lobby id is parsed");
    result |= check(!parseProtocolUrl(QStringLiteral("https://join/123"), nullptr, &error),
                    "non sapie link is rejected");
    result |= check(!parseProtocolUrl(QStringLiteral("sapie://join/not-a-number"), nullptr, &error),
                    "bad lobby id is rejected");

    const QString extended = extendedLengthPath(QStringLiteral("C:/games/example.exe"));
    result |= check(extended.startsWith(QStringLiteral("\\\\?\\C:\\")),
                    "absolute paths use the extended length prefix");

    QTemporaryDir directory;
    result |= check(directory.isValid(), "temporary directory is available");
    const QString tokenPath = QDir(directory.path()).filePath(QStringLiteral("sapie_client.json"));
    result |= check(writeOwnerOnlyFile(tokenPath, QByteArray("token"), &error),
                    "owner-only token file is written");
    result |= check(verifyOwnerOnlyFile(tokenPath, &error),
                    "owner-only token file passes ACL verification");
    result |= check(QFileInfo::exists(tokenPath), "token file exists after the atomic replace");

    if (result == 0) {
        QTextStream(stdout) << "sapie-windows selfcheck passed" << Qt::endl;
    }
    return result;
}
