#include "linux_platform.h"

#include "linux_native.h"

#include <QDBusAbstractAdaptor>
#include <QDBusArgument>
#include <QDBusConnection>
#include <QDBusMessage>
#include <QDBusMetaType>
#include <QGuiApplication>
#include <QIcon>
#include <QLocalServer>
#include <QLocalSocket>
#include <QProcess>
#include <QStyleHints>
#include <QWindow>

namespace sapie::lnx {

struct SniToolTip {
    QString title;
    QString description;
    QString iconName;
    QString iconDescription;
};

}
Q_DECLARE_METATYPE(sapie::lnx::SniToolTip)
namespace sapie::lnx {

QDBusArgument& operator<<(QDBusArgument& argument, const SniToolTip& value)
{
    argument.beginStructure();
    argument << value.title << value.description << value.iconName << value.iconDescription;
    argument.endStructure();
    return argument;
}

const QDBusArgument& operator>>(const QDBusArgument& argument, SniToolTip& value)
{
    argument.beginStructure();
    argument >> value.title >> value.description >> value.iconName >> value.iconDescription;
    argument.endStructure();
    return argument;
}

namespace {

constexpr char kDesktopFile[] = "org.sapie.SAPIE.desktop";
constexpr char kTrayPath[] = "/org/sapie/SAPIE/StatusNotifierItem";

class StatusNotifierAdaptor : public QDBusAbstractAdaptor
{
    Q_OBJECT
    Q_CLASSINFO("D-Bus Interface", "org.kde.StatusNotifierItem")
    Q_PROPERTY(QString Category READ category CONSTANT)
    Q_PROPERTY(QString Id READ id CONSTANT)
    Q_PROPERTY(QString Title READ title CONSTANT)
    Q_PROPERTY(QString Status READ status NOTIFY newStatus)
    Q_PROPERTY(QString IconName READ iconName NOTIFY newIcon)
    Q_PROPERTY(SniToolTip ToolTip READ toolTip CONSTANT)

public:
    StatusNotifierAdaptor(LinuxPlatform* owner, const QString& iconName, QObject* parent)
        : QDBusAbstractAdaptor(parent)
        , m_owner(owner)
        , m_iconName(iconName)
    {
    }

    QString category() const { return QStringLiteral("ApplicationStatus"); }
    QString id() const { return QStringLiteral("SAPIE"); }
    QString title() const { return QStringLiteral("SAPIE"); }
    QString status() const { return QStringLiteral("Active"); }
    QString iconName() const { return m_iconName; }
    SniToolTip toolTip() const
    {
        return {QStringLiteral("SAPIE"), QStringLiteral("Your games. Your server."),
                m_iconName, QString()};
    }

public slots:
    void Activate(int, int) { emit m_owner->trayActivated(true); }
    void SecondaryActivate(int, int) { emit m_owner->trayActivated(false); }
    void ContextMenu(int, int) {}
    void Scroll(int, const QString&) {}

signals:
    void newStatus();
    void newIcon();

private:
    LinuxPlatform* m_owner = nullptr;
    QString m_iconName;
};

QString socketPath()
{
    return runtimeDirectory() + QStringLiteral("/sapie-single-instance.sock");
}

bool canConnect(const QString& path)
{
    QLocalSocket probe;
    probe.connectToServer(path);
    return probe.waitForConnected(300);
}

bool reducedMotionSetting()
{
    QProcess gsettings;
    gsettings.start(QStringLiteral("gsettings"),
                    {QStringLiteral("get"), QStringLiteral("org.gnome.desktop.interface"),
                     QStringLiteral("enable-animations")});
    if (!gsettings.waitForFinished(1000) || gsettings.exitCode() != 0) {
        return false;
    }
    return QString::fromLocal8Bit(gsettings.readAllStandardOutput()).contains(QStringLiteral("false"));
}

bool reject(QString* error, const QString& message)
{
    if (error) {
        *error = message;
    }
    return false;
}

}

LinuxPlatform::LinuxPlatform(QObject* parent)
    : QObject(parent)
    , m_reducedMotion(reducedMotionSetting())
{
    QGuiApplication::setDesktopFileName(QString::fromUtf8(kDesktopFile));
}

LinuxPlatform::~LinuxPlatform() = default;

bool LinuxPlatform::reducedMotion() const
{
    return m_reducedMotion;
}

QString LinuxPlatform::desktopSession() const
{
    if (isWayland()) {
        return QStringLiteral("wayland");
    }
    if (isX11()) {
        return QStringLiteral("x11");
    }
    return {};
}

bool LinuxPlatform::trayAvailable() const
{
    return m_trayHost != nullptr && QDBusConnection::sessionBus().isConnected();
}

bool LinuxPlatform::listenForSecondInstance(QString* error)
{
    const QString path = socketPath();
    if (canConnect(path)) {
        return reject(error, QStringLiteral("Another launcher instance is already running."));
    }
    QLocalServer::removeServer(path);
    auto* server = new QLocalServer(this);
    if (!server->listen(path)) {
        const QString message = server->errorString();
        delete server;
        return reject(error, QStringLiteral("The single-instance socket could not be opened: %1")
                                   .arg(message));
    }
    m_localServer = server;
    connect(server, &QLocalServer::newConnection, this, &LinuxPlatform::onNewLocalConnection);
    return true;
}

bool LinuxPlatform::sendToRunningInstance(const QStringList& arguments, QString* error)
{
    const QString path = socketPath();
    QLocalSocket socket;
    socket.connectToServer(path);
    if (!socket.waitForConnected(1000)) {
        return reject(error, QStringLiteral("No running launcher answered."));
    }
    for (const QString& argument : arguments) {
        if (argument.contains(QLatin1Char('\n'))) {
            return reject(error, QStringLiteral("An argument contains a newline."));
        }
        const QByteArray line = argument.toUtf8() + '\n';
        socket.write(line);
    }
    const bool sent = socket.waitForBytesWritten(1000);
    socket.disconnectFromServer();
    if (!sent) {
        return reject(error, QStringLiteral("The running launcher did not accept the arguments."));
    }
    return true;
}

void LinuxPlatform::attachWindow(QWindow* window)
{
    if (!window) {
        return;
    }
    const QIcon icon = QIcon::fromTheme(QStringLiteral("org.sapie.SAPIE"));
    if (!icon.isNull()) {
        window->setIcon(icon);
    }
}

void LinuxPlatform::installTray(const QString& iconName)
{
    QDBusConnection bus = QDBusConnection::sessionBus();
    if (!bus.isConnected() || m_trayHost) {
        return;
    }
    qRegisterMetaType<SniToolTip>("SniToolTip");
    qDBusRegisterMetaType<SniToolTip>();

    auto* host = new QObject(this);
    new StatusNotifierAdaptor(this, iconName, host);
    if (!bus.registerObject(QString::fromUtf8(kTrayPath), host,
                            QDBusConnection::ExportAdaptors)) {
        host->deleteLater();
        return;
    }
    m_trayHost = host;

    for (const QString& watcher : {QStringLiteral("org.freedesktop.StatusNotifierWatcher"),
                                   QStringLiteral("org.kde.StatusNotifierWatcher")}) {
        QDBusMessage call = QDBusMessage::createMethodCall(
            watcher, QStringLiteral("/StatusNotifierWatcher"), watcher,
            QStringLiteral("RegisterStatusNotifierItem"));
        call << QString::fromUtf8(kTrayPath);
        bus.call(call, QDBus::NoBlock);
    }
}

void LinuxPlatform::onNewLocalConnection()
{
    auto* server = static_cast<QLocalServer*>(m_localServer);
    if (!server) {
        return;
    }
    while (QLocalSocket* socket = server->nextPendingConnection()) {
        connect(socket, &QLocalSocket::readyRead, this, [this, socket]() {
            m_incoming += socket->readAll();
            int newline = m_incoming.indexOf('\n');
            while (newline >= 0) {
                const QString argument = QString::fromUtf8(m_incoming.left(newline));
                m_incoming.remove(0, newline + 1);
                if (!argument.isEmpty()) {
                    QString lobby;
                    QString error;
                    if (parseProtocolUrl(argument, &lobby, &error)) {
                        emit protocolLinkReceived(argument);
                    } else {
                        emit secondInstanceLaunched({argument});
                    }
                }
                newline = m_incoming.indexOf('\n');
            }
        });
        connect(socket, &QLocalSocket::disconnected, socket, &QObject::deleteLater);
    }
}

}

#include "linux_platform.moc"
