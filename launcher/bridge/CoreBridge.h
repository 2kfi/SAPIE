#pragma once

#include <QObject>
#include <QString>
#include <QVariantList>
#include <QVariantMap>

struct SapieCore;

class CoreBridge : public QObject
{
    Q_OBJECT
    Q_PROPERTY(quint32 abiVersion READ abiVersion CONSTANT)
    Q_PROPERTY(QString lastError READ lastError NOTIFY errorChanged)

public:
    explicit CoreBridge(QObject *parent = nullptr);
    ~CoreBridge() override;

    quint32 abiVersion() const;
    QString lastError() const;

    Q_INVOKABLE void shutdown();
    Q_INVOKABLE bool createCore(quint32 abiVersion, bool fake = false);
    Q_INVOKABLE bool testConnection(const QString &server);
    Q_INVOKABLE QVariantMap login(const QString &server, const QString &username,
                                  const QString &password);
    Q_INVOKABLE bool signOut();
    Q_INVOKABLE QVariantMap detectGame(const QString &path);
    Q_INVOKABLE QVariantMap installLibrary(const QString &path);
    Q_INVOKABLE QString restoreLibrary(const QString &path);
    Q_INVOKABLE QVariantMap configureGame(const QString &path, quint32 appid);
    Q_INVOKABLE quint32 launchGame(const QString &path);
    Q_INVOKABLE QString unpackSteamStub(const QString &path);
    Q_INVOKABLE QVariantList libraryList();
    Q_INVOKABLE QVariantMap libraryAdd(const QString &path);
    Q_INVOKABLE bool libraryRemove(const QString &path);

signals:
    void progress(const QString &step, const QString &message);
    void errorOccurred(quint32 status, const QString &message);
    void errorChanged();

private:
    bool recordStatus(quint32 status, const QString &action);
    void setLastError(const QString &message);
    void destroyCore();
    static void handleEvent(void *userData, const char *step, const char *message);

    SapieCore *m_core = nullptr;
    QString m_lastError;
};
