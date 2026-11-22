#include "LauncherShell.h"

#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QFileDialog>
#include <QFileInfo>
#include <QGuiApplication>
#include <QRegularExpression>
#include <QScreen>
#include <QSettings>
#include <QStandardPaths>
#include <QLocalServer>
#include <QLocalSocket>
#include <QQuickWindow>

#include <cstdio>
#include <cstdlib>
#include <memory>

namespace {
void messageHandler(QtMsgType type, const QMessageLogContext &context, const QString &message)
{
    Q_UNUSED(context)

    const QString timestamp = QDateTime::currentDateTime().toString(Qt::ISODateWithMs);
    const QString line = QStringLiteral("%1 %2\n").arg(timestamp, LauncherShell::redact(message));
    const QString path = launcherLogPath();
    QFile file(path);
    if (QDir().mkpath(QFileInfo(path).absolutePath()) && file.open(QIODevice::Append)) {
        file.write(line.toUtf8());
        file.flush();
    }
    std::fprintf(stderr, "%s", qPrintable(line));

    if (type == QtFatalMsg) {
        std::abort();
    }
}
} // namespace

QString launcherLogPath()
{
    return QStandardPaths::writableLocation(QStandardPaths::AppLocalDataLocation)
            + QStringLiteral("/logs/sapie-launcher.log");
}

void installLauncherLogging()
{
    qInstallMessageHandler(messageHandler);
}

LauncherShell::LauncherShell(QObject *parent)
    : QObject(parent)
    , m_instanceServer(new QLocalServer(this))
{
}

LauncherShell::~LauncherShell()
{
    m_instanceServer->close();
}

QString LauncherShell::redact(const QString &message)
{
    QString result = message;
    const QRegularExpression quoted(QStringLiteral(
            "(?i)(\"?(?:password|token|session_token|secret)\"?\\s*[:=]\\s*)\"[^\"]*\""));
    result.replace(quoted, QStringLiteral("\\1\"[redacted]\""));

    const QRegularExpression bare(QStringLiteral(
            "(?i)\\b(password|token|session_token|secret)\\s*[:=]\\s*[^\\s,;&]+"));
    result.replace(bare, QStringLiteral("\\1=[redacted]"));
    return result;
}

QString LauncherShell::inviteFromArguments(const QStringList &arguments)
{
    const QString prefix = QStringLiteral("sapie://join/");
    for (const QString &argument : arguments) {
        if (argument.startsWith(prefix) && !argument.contains(QLatin1Char('\n'))
            && !argument.contains(QLatin1Char('\r'))) {
            return argument;
        }
    }
    return {};
}

QString LauncherShell::pickExecutable()
{
    return QFileDialog::getOpenFileName(nullptr, QStringLiteral("Choose executable"),
                                        QDir::homePath(), QStringLiteral("All files (*)"));
}

bool LauncherShell::acquireSingleInstance(const QString &inviteUrl)
{
    const QString name = QStringLiteral("sapie-launcher");
    QLocalSocket probe(this);
    probe.connectToServer(name);
    if (probe.waitForConnected(200)) {
        const QByteArray payload = inviteUrl.isEmpty()
                ? QByteArray("raise")
                : QByteArray("join ") + inviteUrl.toUtf8() + '\n';
        probe.write(payload);
        probe.waitForBytesWritten(200);
        return false;
    }

    QLocalServer::removeServer(name);
    m_instanceServer->setSocketOptions(QLocalServer::UserAccessOption);
    if (!m_instanceServer->listen(name)) {
        qWarning() << "Could not reserve the launcher instance:" << m_instanceServer->errorString();
        return false;
    }
    connect(m_instanceServer, &QLocalServer::newConnection, this,
            &LauncherShell::onInstanceConnection);
    return true;
}

void LauncherShell::onInstanceConnection()
{
    while (m_instanceServer->hasPendingConnections()) {
        QLocalSocket *socket = m_instanceServer->nextPendingConnection();
        auto buffer = std::make_shared<QByteArray>();
        const auto read = [this, socket, buffer]() {
            buffer->append(socket->readAll());
            if (*buffer == "raise") {
                socket->disconnectFromServer();
                return;
            }
            const int split = buffer->indexOf('\n');
            if (split < 0) {
                if (buffer->size() > 4096) {
                    socket->disconnectFromServer();
                }
                return;
            }
            if (buffer->startsWith("join ")) {
                const QString url = QString::fromUtf8(buffer->constData() + 5, split - 5);
                if (!url.isEmpty()) {
                    emit inviteReceived(url);
                }
            }
            socket->disconnectFromServer();
        };
        connect(socket, &QLocalSocket::readyRead, this, read);
        read();
    }
}

void LauncherShell::restoreWindow(QQuickWindow *window)
{
    if (window == nullptr) {
        return;
    }

    QSettings settings(QStringLiteral("SAPIE"), QStringLiteral("SAPIE Launcher"));
    const QString key = QStringLiteral("window/");
    if (!settings.contains(key + QStringLiteral("x")) || !settings.contains(key + QStringLiteral("y"))
        || !settings.contains(key + QStringLiteral("width"))
        || !settings.contains(key + QStringLiteral("height"))) {
        return;
    }

    QRect geometry(settings.value(key + QStringLiteral("x")).toInt(),
                   settings.value(key + QStringLiteral("y")).toInt(),
                   settings.value(key + QStringLiteral("width")).toInt(),
                   settings.value(key + QStringLiteral("height")).toInt());
    QScreen *screen = window->screen() == nullptr ? QGuiApplication::primaryScreen()
                                                  : window->screen();
    if (screen != nullptr && geometry.isValid()) {
        const QRect available = screen->availableGeometry();
        if (!geometry.intersects(available)) {
            geometry.moveCenter(available.center());
        }
        geometry = geometry.intersected(available);
        if (geometry.width() < window->minimumWidth()
            || geometry.height() < window->minimumHeight()) {
            geometry = available.adjusted(40, 40, -40, -40);
        }
    }
    if (geometry.isValid()) {
        window->setGeometry(geometry);
    }
    if (settings.value(key + QStringLiteral("maximized")).toBool()) {
        window->showMaximized();
    }
}

void LauncherShell::saveWindow(QQuickWindow *window)
{
    if (window == nullptr || window->visibility() == QWindow::Minimized) {
        return;
    }

    QSettings settings(QStringLiteral("SAPIE"), QStringLiteral("SAPIE Launcher"));
    const QString key = QStringLiteral("window/");
    settings.setValue(key + QStringLiteral("x"), window->x());
    settings.setValue(key + QStringLiteral("y"), window->y());
    settings.setValue(key + QStringLiteral("width"), window->width());
    settings.setValue(key + QStringLiteral("height"), window->height());
    settings.setValue(key + QStringLiteral("maximized"),
                      window->visibility() == QWindow::Maximized);
    settings.sync();
}
