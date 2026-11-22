#pragma once

#include <QMutex>
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

public:
    QVector<QByteArray> requests() const;

signals:
    void portReady(quint16 port);

private slots:
    void onNewConnection();

private:
    QTcpServer m_server;
    QVector<QTcpSocket *> m_sockets;
    mutable QMutex m_mutex;
    QVector<QByteArray> m_requests;
};

class CoreBridgeTest : public QObject
{
    Q_OBJECT

private slots:
    void init();
    void unknownAbiVersionHasClearError();
    void fakeCoreDrivesLoginDetectConfigureAndLaunch();
    void fakeCoreDrivesScreenMethods();
    void fakeCoreDrivesServerSocialMethods();
    void fakePlatformRecordsNativeCalls();
};
