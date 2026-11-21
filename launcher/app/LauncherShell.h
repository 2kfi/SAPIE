#pragma once

#include <QObject>
#include <QString>

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

    bool acquireSingleInstance();
    void restoreWindow(QQuickWindow *window);
    void saveWindow(QQuickWindow *window);

private:
    QLocalServer *m_instanceServer = nullptr;
};
