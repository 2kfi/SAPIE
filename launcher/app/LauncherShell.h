#pragma once

#include <QObject>
#include <QString>
#include <QStringList>

class QLocalServer;
class QQuickWindow;

void installLauncherLogging();
QString launcherLogPath();

class LauncherShell : public QObject
{
    Q_OBJECT

public:
    explicit LauncherShell(QObject *parent = nullptr);
    ~LauncherShell() override;

    static QString redact(const QString &message);
    static QString inviteFromArguments(const QStringList &arguments);
    static QString pickExecutable();

    bool acquireSingleInstance(const QString &inviteUrl = QString());
    void restoreWindow(QQuickWindow *window);
    void saveWindow(QQuickWindow *window);

signals:
    void inviteReceived(const QString &url);

private:
    void onInstanceConnection();

    QLocalServer *m_instanceServer = nullptr;
};
