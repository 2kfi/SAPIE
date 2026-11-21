#include "linux_native.h"

#include <QDir>
#include <QFileInfo>
#include <QProcess>
#include <QRegularExpression>
#include <QStandardPaths>
#include <QUuid>
#include <QUrl>

namespace sapie::lnx {
namespace {

QString xdgPath(const char* variable, const QString& fallback)
{
    const QByteArray fromEnv = qgetenv(variable);
    const QString value = QString::fromLocal8Bit(fromEnv.constData());
    if (value.isEmpty() || !QDir::isAbsolutePath(value)) {
        return QDir::cleanPath(QDir::homePath() + QLatin1Char('/') + fallback);
    }
    return QDir::cleanPath(value);
}

QString ensureDirectory(const QString& path)
{
    if (!QDir().mkpath(path)) {
        return {};
    }
    QFile::setPermissions(path, QFileDevice::ReadOwner | QFileDevice::WriteOwner
                          | QFileDevice::ExeOwner);
    return path;
}

bool reject(QString* error, const QString& message)
{
    if (error) {
        *error = message;
    }
    return false;
}

}

QString dataDirectory()
{
    return ensureDirectory(xdgPath("XDG_DATA_HOME", QStringLiteral(".local/share"))
                           + QStringLiteral("/sapie"));
}

QString configDirectory()
{
    return ensureDirectory(xdgPath("XDG_CONFIG_HOME", QStringLiteral(".config"))
                           + QStringLiteral("/sapie"));
}

QString cacheDirectory()
{
    return ensureDirectory(xdgPath("XDG_CACHE_HOME", QStringLiteral(".cache"))
                           + QStringLiteral("/sapie"));
}

QString runtimeDirectory()
{
    const QByteArray fromEnv = qgetenv("XDG_RUNTIME_DIR");
    const QString root = QString::fromLocal8Bit(fromEnv.constData());
    if (!root.isEmpty() && QDir::isAbsolutePath(root)) {
        return ensureDirectory(QDir::cleanPath(root + QStringLiteral("/sapie")));
    }
    return cacheDirectory();
}

QString tokenFilePath()
{
    return runtimeDirectory() + QStringLiteral("/sapie_client.json");
}

bool parseProtocolUrl(const QString& argument, QString* lobbyId, QString* error)
{
    const QUrl url(argument.trimmed());
    if (!url.isValid() || url.scheme().compare(QStringLiteral("sapie"), Qt::CaseInsensitive) != 0) {
        return reject(error, QStringLiteral("The link is not a sapie link."));
    }
    if (!url.userInfo().isEmpty() || url.port() != -1 || !url.fragment().isEmpty()) {
        return reject(error, QStringLiteral("The link contains an unsupported part."));
    }

    QString id;
    if (url.host().compare(QStringLiteral("join"), Qt::CaseInsensitive) == 0) {
        const QStringList parts = url.path().split(QLatin1Char('/'), Qt::SkipEmptyParts);
        if (parts.size() != 1) {
            return reject(error, QStringLiteral("The invite link has no lobby id."));
        }
        id = parts.first();
    } else if (url.host().isEmpty()
               && url.path().startsWith(QStringLiteral("/join/"), Qt::CaseInsensitive)) {
        id = url.path().mid(6);
        if (id.contains(QLatin1Char('/'))) {
            return reject(error, QStringLiteral("The invite link has an invalid lobby id."));
        }
    } else {
        return reject(error, QStringLiteral("The link is not a lobby invite."));
    }

    static const QRegularExpression numeric(QStringLiteral("^[0-9]{1,20}$"));
    if (!numeric.match(id).hasMatch()) {
        return reject(error, QStringLiteral("The lobby id must be a number."));
    }
    if (lobbyId) {
        *lobbyId = id;
    }
    return true;
}

bool registerMimeTypes(const QStringList& mimeTypes, QString* error)
{
    const QString xdgMime = QStandardPaths::findExecutable(QStringLiteral("xdg-mime"));
    if (xdgMime.isEmpty()) {
        return reject(error, QStringLiteral("xdg-mime is not installed."));
    }
    static const QRegularExpression validMime(
        QStringLiteral("^[A-Za-z0-9!#$&^_.+-]+/[A-Za-z0-9!#$&^_.+-]+$"));
    for (const QString& mime : mimeTypes) {
        if (!validMime.match(mime).hasMatch()) {
            return reject(error, QStringLiteral("Invalid MIME type: %1").arg(mime));
        }
        QProcess process;
        process.start(xdgMime, {QStringLiteral("default"),
                                QStringLiteral("org.sapie.SAPIE.desktop"), mime});
        if (!process.waitForFinished(5000) || process.exitStatus() != QProcess::NormalExit
            || process.exitCode() != 0) {
            const QString detail = QString::fromLocal8Bit(process.readAllStandardError()).trimmed();
            return reject(error, QStringLiteral("Could not set the default for %1. %2")
                                       .arg(mime, detail));
        }
    }
    return true;
}

bool registerProtocol(QString* error)
{
    return registerMimeTypes({QStringLiteral("x-scheme-handler/sapie")}, error);
}

bool buildLaunchPlan(const QString& executable, const QString& preload,
                     const QStringList& arguments, LaunchPlan* plan, QString* error)
{
    if (!plan) {
        return reject(error, QStringLiteral("No launch plan was supplied."));
    }
    const QFileInfo exe(executable);
    if (!QDir::isAbsolutePath(executable) || !exe.isFile() || !exe.isExecutable()) {
        return reject(error, QStringLiteral("The game executable is missing or not executable."));
    }

    LaunchPlan result;
    result.program = exe.absoluteFilePath();
    result.arguments = arguments;
    if (!preload.isEmpty()) {
        const QFileInfo library(preload);
        if (!QDir::isAbsolutePath(preload) || !library.isFile()) {
            return reject(error, QStringLiteral("The preload library is not an existing file."));
        }
    }

    result.environment = QProcessEnvironment::systemEnvironment();
    result.environment.insert(QStringLiteral("SAPIE_LAUNCH"), QStringLiteral("1"));
    if (!preload.isEmpty()) {
        const QString existing = result.environment.value(QStringLiteral("LD_PRELOAD"));
        result.environment.insert(QStringLiteral("LD_PRELOAD"),
                                  existing.isEmpty() ? preload : preload + QLatin1Char(' ') + existing);
    }
    *plan = result;
    return true;
}

bool startGame(const LaunchPlan& plan, qint64* pid, QString* error)
{
    QProcess process;
    process.setProgram(plan.program);
    process.setArguments(plan.arguments);
    process.setProcessEnvironment(plan.environment);
    if (!process.startDetached(pid)) {
        return reject(error, QStringLiteral("The game process could not be started."));
    }
    return true;
}

bool isWayland()
{
    const QByteArray session = qgetenv("XDG_SESSION_TYPE");
    return session == "wayland" || !qgetenv("WAYLAND_DISPLAY").isEmpty();
}

bool isX11()
{
    const QByteArray session = qgetenv("XDG_SESSION_TYPE");
    return session == "x11" || (session.isEmpty() && !qgetenv("DISPLAY").isEmpty());
}

}
