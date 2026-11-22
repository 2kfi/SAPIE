#pragma once

#include <QByteArray>
#include <QObject>
#include <QString>
#include <QStringList>

class QWindow;

namespace sapie::lnx {

class LinuxPlatform : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool reducedMotion READ reducedMotion CONSTANT)
    Q_PROPERTY(QString desktopSession READ desktopSession CONSTANT)
    Q_PROPERTY(bool trayAvailable READ trayAvailable CONSTANT)

public:
    explicit LinuxPlatform(QObject* parent = nullptr);
    ~LinuxPlatform() override;

    bool reducedMotion() const;
    QString desktopSession() const;
    bool trayAvailable() const;

    bool listenForSecondInstance(QString* error);
    static bool sendToRunningInstance(const QStringList& arguments, QString* error);

    void attachWindow(QWindow* window);
    void installTray(const QString& iconName);

signals:
    void protocolLinkReceived(const QString& url);
    void secondInstanceLaunched(const QStringList& arguments);
    void trayActivated(bool leftClick);

private slots:
    void onNewLocalConnection();

private:
    void* m_localServer = nullptr;
    void* m_trayHost = nullptr;
    QByteArray m_incoming;
    bool m_reducedMotion = false;
};

}
