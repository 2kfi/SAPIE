#include "win_native.h"

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QRegularExpression>
#include <QSettings>
#include <QUrl>
#include <QWindow>

#include <windows.h>
#include <dwmapi.h>
#include <shobjidl_core.h>

#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

namespace sapie::win {
namespace {

QString setError(QString* error, const QString& message)
{
    if (error) {
        *error = message;
    }
    return {};
}

QString applicationCommand()
{
    const QString exe = QCoreApplication::applicationFilePath();
    return QStringLiteral("\"%1\" \"%%1\"").arg(exe);
}

}

QString dataDirectory()
{
    const QByteArray fromEnv = qgetenv("LOCALAPPDATA");
    QString root = QString::fromLocal8Bit(fromEnv.constData());
    if (root.isEmpty()) {
        root = QDir::homePath() + QStringLiteral("/AppData/Local");
    }
    const QString path = QDir::cleanPath(root + QStringLiteral("/SAPIE"));
    QDir().mkpath(path);
    return path;
}

QString extendedLengthPath(const QString& path)
{
    QString native = QDir::toNativeSeparators(QFileInfo(path).absoluteFilePath());
    if (native.startsWith(QStringLiteral("\\\\?\\"))) {
        return native;
    }
    if (native.startsWith(QStringLiteral("\\\\"))) {
        return QStringLiteral("\\\\?\\UNC\\") + native.mid(2);
    }
    return QStringLiteral("\\\\?\\") + native;
}

bool parseProtocolUrl(const QString& argument, QString* lobbyId, QString* error)
{
    const QString value = argument.trimmed();
    const QUrl url(value);
    if (!url.isValid() || url.scheme().compare(QStringLiteral("sapie"), Qt::CaseInsensitive) != 0) {
        setError(error, QStringLiteral("The link is not a sapie link."));
        return false;
    }
    if (!url.userInfo().isEmpty() || url.port() != -1 || !url.fragment().isEmpty()) {
        setError(error, QStringLiteral("The link contains an unsupported part."));
        return false;
    }

    QString id;
    if (url.host().compare(QStringLiteral("join"), Qt::CaseInsensitive) == 0) {
        const QStringList parts = url.path().split(QLatin1Char('/'), Qt::SkipEmptyParts);
        if (parts.size() != 1) {
            setError(error, QStringLiteral("The invite link has no lobby id."));
            return false;
        }
        id = parts.first();
    } else if (url.host().isEmpty() && url.path().startsWith(QStringLiteral("/join/"), Qt::CaseInsensitive)) {
        id = url.path().mid(6);
        if (id.contains(QLatin1Char('/'))) {
            setError(error, QStringLiteral("The invite link has an invalid lobby id."));
            return false;
        }
    } else {
        setError(error, QStringLiteral("The link is not a lobby invite."));
        return false;
    }

    static const QRegularExpression numeric(QStringLiteral("^[0-9]{1,20}$"));
    if (!numeric.match(id).hasMatch()) {
        setError(error, QStringLiteral("The lobby id must be a number."));
        return false;
    }
    if (lobbyId) {
        *lobbyId = id;
    }
    return true;
}

bool registerProtocol(QString* error)
{
    const QString exe = QCoreApplication::applicationFilePath();
    if (exe.isEmpty() || !QFileInfo::exists(exe)) {
        setError(error, QStringLiteral("The launcher executable path is not available."));
        return false;
    }

    QSettings key(QStringLiteral("HKEY_CURRENT_USER\\Software\\Classes\\sapie"), QSettings::NativeFormat);
    key.setValue(QStringLiteral("."), QStringLiteral("URL:SAPIE protocol"));
    key.setValue(QStringLiteral("URL Protocol"), QString());
    key.setValue(QStringLiteral("DefaultIcon/."), QStringLiteral("\"%1\",0").arg(exe));
    key.setValue(QStringLiteral("shell/open/command/."), applicationCommand());
    key.sync();
    if (key.status() != QSettings::NoError) {
        setError(error, QStringLiteral("Could not register the sapie protocol for this user."));
        return false;
    }
    return true;
}

bool unregisterProtocol(QString* error)
{
    QSettings key(QStringLiteral("HKEY_CURRENT_USER\\Software\\Classes\\sapie"), QSettings::NativeFormat);
    key.remove(QStringLiteral("."));
    key.remove(QStringLiteral("URL Protocol"));
    key.remove(QStringLiteral("DefaultIcon"));
    key.remove(QStringLiteral("shell"));
    key.sync();
    if (key.status() != QSettings::NoError) {
        setError(error, QStringLiteral("Could not remove the sapie protocol registration."));
        return false;
    }
    return true;
}

bool registerFileAssociations(const QStringList& extensions, QString* error)
{
    const QString command = applicationCommand();
    for (QString extension : extensions) {
        if (extension.startsWith(QLatin1Char('*'))) {
            extension.remove(0, 1);
        }
        if (extension.startsWith(QLatin1Char('.'))) {
            extension.remove(0, 1);
        }
        static const QRegularExpression valid(QStringLiteral("^[A-Za-z0-9_-]{1,32}$"));
        if (!valid.match(extension).hasMatch()) {
            setError(error, QStringLiteral("Invalid file extension: %1").arg(extension));
            return false;
        }
        const QString progId = QStringLiteral("sapie.%1").arg(extension);

        QSettings association(
            QStringLiteral("HKEY_CURRENT_USER\\Software\\Classes\\.%1").arg(extension),
            QSettings::NativeFormat);
        association.setValue(QStringLiteral("."), progId);

        QSettings handler(
            QStringLiteral("HKEY_CURRENT_USER\\Software\\Classes\\%1").arg(progId),
            QSettings::NativeFormat);
        handler.setValue(QStringLiteral("."), QStringLiteral("SAPIE file"));
        handler.setValue(QStringLiteral("shell/open/command/."), command);
        handler.sync();
        if (handler.status() != QSettings::NoError) {
            setError(error, QStringLiteral("Could not register the .%1 file association.").arg(extension));
            return false;
        }
    }
    return true;
}

bool unregisterFileAssociations(const QStringList& extensions, QString* error)
{
    for (QString extension : extensions) {
        if (extension.startsWith(QLatin1Char('*'))) {
            extension.remove(0, 1);
        }
        if (extension.startsWith(QLatin1Char('.'))) {
            extension.remove(0, 1);
        }
        QSettings association(
            QStringLiteral("HKEY_CURRENT_USER\\Software\\Classes\\.%1").arg(extension),
            QSettings::NativeFormat);
        association.remove(QStringLiteral("."));
        QSettings handler(
            QStringLiteral("HKEY_CURRENT_USER\\Software\\Classes\\sapie.%1").arg(extension),
            QSettings::NativeFormat);
        handler.remove(QStringLiteral("."));
        handler.remove(QStringLiteral("shell"));
    }
    return true;
}

void applyDarkTitleBar(QWindow* window)
{
    if (!window) {
        return;
    }
    const HWND handle = reinterpret_cast<HWND>(window->winId());
    BOOL dark = TRUE;
    if (FAILED(DwmSetWindowAttribute(handle, DWMWA_USE_IMMERSIVE_DARK_MODE, &dark, sizeof(dark)))) {
        const DWORD legacyAttribute = 19;
        DwmSetWindowAttribute(handle, legacyAttribute, &dark, sizeof(dark));
    }
}

void setAppUserModelId()
{
    SetCurrentProcessExplicitAppUserModelID(L"SAPIE.Launcher.0.5");
}

}
