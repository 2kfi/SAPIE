#pragma once

#include <QObject>
#include <QTcpServer>
#include <QVector>

class QTcpSocket;

class LoginServer : public QObject
{
    Q_OBJECT

public slots:
    void start();
    void stop();

signals:
    void portReady(quint16 port);

private slots:
    void onNewConnection();

private:
    QTcpServer m_server;
    QVector<QTcpSocket *> m_sockets;
};

class CoreBridgeTest : public QObject
{
    Q_OBJECT

private slots:
    void init();
    void unknownAbiVersionHasClearError();
    void fakeCoreDrivesLoginDetectConfigureAndLaunch();
    void fakePlatformRecordsNativeCalls();
};
