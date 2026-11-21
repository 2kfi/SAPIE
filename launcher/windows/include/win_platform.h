#pragma once

#include <QIcon>
#include <QObject>
#include <QString>
#include <QStringList>
#include <QVector>

class QMenu;
class QSystemTrayIcon;
class QWindow;

namespace sapie::win {

struct JumpTask {
    QString title;
    QString executable;
    QString arguments;
    QString icon;
};

class WindowsPlatform : public QObject {
    Q_OBJECT
    Q_PROPERTY(bool reducedMotion READ reducedMotion CONSTANT)
    Q_PROPERTY(QString dataDirectory READ dataDirectory CONSTANT)

public:
    explicit WindowsPlatform(QObject* parent = nullptr);
    ~WindowsPlatform() override;

    bool reducedMotion() const;
    QString dataDirectory() const;

    bool registerIntegrations(const QStringList& fileExtensions, QString* error);
    bool unregisterIntegrations(const QStringList& fileExtensions, QString* error);

    void attachWindow(QWindow* window);
    void installTray(const QIcon& icon, QMenu* menu);
    void setJumpList(const QVector<JumpTask>& tasks, QString* error);
    void setProgress(int value, int maximum);
    void clearProgress();

    bool listenForSecondInstance(QString* error);
    static bool sendToRunningInstance(const QStringList& arguments, QString* error);

signals:
    void protocolLinkReceived(const QString& url);
    void secondInstanceLaunched(const QStringList& arguments);
    void trayActivated(bool leftClick);

private slots:
    void onNewLocalConnection();

private:
    QString currentSid() const;

    QSystemTrayIcon* m_tray = nullptr;
    void* m_taskbar = nullptr;
    void* m_localServer = nullptr;
    void* m_window = nullptr;
    bool m_comInitialised = false;
};

}
