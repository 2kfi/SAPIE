#include "linux_native.h"
#include "linux_platform.h"
#include "linux_token.h"
#include "linux_updater.h"

#include <QCoreApplication>
#include <QDir>
#include <QEventLoop>
#include <QFile>
#include <QTemporaryDir>
#include <QTextStream>
#include <QTimer>

namespace {

int failures = 0;

int check(bool condition, const char* message)
{
    if (!condition) {
        QTextStream(stderr) << "FAIL: " << message << Qt::endl;
        ++failures;
    }
    return failures;
}

QByteArray manifestBody(const QString& channel, const QString& hash)
{
    return ("{\"schema\":\"sapie-update/v1\",\"channel\":\"" + channel
            + "\",\"version\":\"0.5.1\",\"artifacts\":[{\"name\":\"sapie-launcher-0.5.1-linux-x64.AppImage\","
              "\"os\":\"linux\",\"arch\":\"x64\",\"sha256\":\""
            + hash
            + "\",\"size\":1234,\"url\":\"https://example.invalid/sapie-launcher-0.5.1-linux-x64.AppImage\"}]}")
               .toUtf8();
}

}

int main(int argc, char** argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir root;
    if (!root.isValid()) {
        QTextStream(stderr) << "FAIL: temporary directory is unavailable" << Qt::endl;
        return 1;
    }
    qputenv("XDG_DATA_HOME", (root.path() + "/data").toUtf8());
    qputenv("XDG_CONFIG_HOME", (root.path() + "/config").toUtf8());
    qputenv("XDG_CACHE_HOME", (root.path() + "/cache").toUtf8());
    qputenv("XDG_RUNTIME_DIR", (root.path() + "/runtime").toUtf8());
    QDir().mkpath(root.path() + "/runtime");

    QString lobby;
    QString error;
    check(sapie::lnx::parseProtocolUrl(QStringLiteral("sapie://join/123?exp=1&sig=abc"),
                                       &lobby, &error),
          "valid invite link is accepted");
    check(lobby == QStringLiteral("123"), "invite lobby id is parsed");
    check(!sapie::lnx::parseProtocolUrl(QStringLiteral("https://join/123"), nullptr, &error),
          "non sapie link is rejected");
    check(!sapie::lnx::parseProtocolUrl(QStringLiteral("sapie://join/not-a-number"), nullptr, &error),
          "bad lobby id is rejected");
    check(!sapie::lnx::parseProtocolUrl(QStringLiteral("sapie://join/123#x"), nullptr, &error),
          "fragment is rejected");

    check(sapie::lnx::dataDirectory().endsWith(QStringLiteral("/data/sapie")),
          "XDG data directory is used");
    check(sapie::lnx::configDirectory().endsWith(QStringLiteral("/config/sapie")),
          "XDG config directory is used");
    check(sapie::lnx::runtimeDirectory().endsWith(QStringLiteral("/runtime/sapie")),
          "XDG runtime directory is used");

    const QString tokenPath = sapie::lnx::runtimeDirectory() + QStringLiteral("/token.json");
    check(sapie::lnx::writeOwnerOnlyFile(tokenPath, QByteArray("token"), &error),
          "owner-only token is written");
    check(sapie::lnx::verifyOwnerOnlyFile(tokenPath, &error), "0600 token passes verification");
    QFile writtenToken(tokenPath);
    check(writtenToken.open(QIODevice::ReadOnly)
          && writtenToken.readAll() == QByteArray("token"),
          "token content survives the atomic replace");
    QFile::setPermissions(tokenPath, QFileDevice::ReadOwner | QFileDevice::WriteOwner
                          | QFileDevice::ReadGroup);
    check(!sapie::lnx::verifyOwnerOnlyFile(tokenPath, &error), "group-readable token is rejected");

    const QString preload = root.path() + QStringLiteral("/libfake.so");
    {
        QFile library(preload);
        if (library.open(QIODevice::WriteOnly)) {
            library.write("x");
        }
    }
    sapie::lnx::LaunchPlan plan;
    check(sapie::lnx::buildLaunchPlan(QStringLiteral("/bin/true"), preload,
                                      {QStringLiteral("--arg")}, &plan, &error),
          "launch plan is built");
    check(plan.environment.value(QStringLiteral("LD_PRELOAD")) == preload,
          "LD_PRELOAD is set for the game process");
    check(plan.arguments == QStringList{QStringLiteral("--arg")}, "game arguments are passed verbatim");
    check(!sapie::lnx::buildLaunchPlan(QStringLiteral("true"), preload, {}, &plan, &error),
          "relative executable path is rejected");
    check(!sapie::lnx::buildLaunchPlan(QStringLiteral("/bin/true"),
                                       root.path() + QStringLiteral("/missing.so"), {}, &plan, &error),
          "missing preload library is rejected");

    const QString validHash(64, QLatin1Char('a'));
    sapie::lnx::UpdateManifest manifest;
    check(sapie::lnx::parseUpdateManifest(manifestBody(QStringLiteral("stable"), validHash),
                                          QStringLiteral("stable"), &manifest, &error),
          "signed manifest shape parses");
    check(!sapie::lnx::parseUpdateManifest(manifestBody(QStringLiteral("beta"), validHash),
                                           QStringLiteral("stable"), &manifest, &error),
          "channel mismatch is rejected");
    check(!sapie::lnx::parseUpdateManifest(manifestBody(QStringLiteral("stable"), QStringLiteral("bad")),
                                           QStringLiteral("stable"), &manifest, &error),
          "bad artifact hash is rejected");

    qputenv("XDG_SESSION_TYPE", "wayland");
    check(sapie::lnx::isWayland(), "Wayland session is detected");
    qunsetenv("WAYLAND_DISPLAY");
    qputenv("XDG_SESSION_TYPE", "x11");
    check(sapie::lnx::isX11() && !sapie::lnx::isWayland(), "X11 session is detected");

    sapie::lnx::LinuxPlatform platform;
    check(platform.desktopSession() == QStringLiteral("x11"), "desktop session is reported");
    check(platform.listenForSecondInstance(&error), "single-instance socket is opened");
    QString receivedLink;
    QObject::connect(&platform, &sapie::lnx::LinuxPlatform::protocolLinkReceived,
                     &app, [&receivedLink](const QString& url) { receivedLink = url; });
    check(sapie::lnx::LinuxPlatform::sendToRunningInstance(
              {QStringLiteral("sapie://join/123")}, &error),
          "second instance forwards its arguments");
    QEventLoop loop;
    QTimer::singleShot(500, &loop, &QEventLoop::quit);
    loop.exec();
    check(receivedLink == QStringLiteral("sapie://join/123"), "forwarded invite reaches the running instance");

    if (failures == 0) {
        QTextStream(stdout) << "sapie-linux selfcheck passed" << Qt::endl;
    }
    return failures == 0 ? 0 : 1;
}
