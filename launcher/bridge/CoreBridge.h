#pragma once

#include <QObject>
#include <QString>
#include <QVariantList>
#include <QVariantMap>

#include <functional>

struct SapieCore;

class CoreBridge : public QObject
{
    Q_OBJECT
    Q_PROPERTY(bool isFake READ isFake CONSTANT)
    Q_PROPERTY(quint32 abiVersion READ abiVersion CONSTANT)
    Q_PROPERTY(QString lastError READ lastError NOTIFY errorChanged)

    Q_PROPERTY(QString connectionState MEMBER m_connectionState NOTIFY connectionStateChanged)
    Q_PROPERTY(QString serverAddress MEMBER m_serverAddress NOTIFY serverAddressChanged)
    Q_PROPERTY(QString accountName MEMBER m_accountName NOTIFY accountNameChanged)
    Q_PROPERTY(bool rememberSession MEMBER m_rememberSession NOTIFY rememberSessionChanged)
    Q_PROPERTY(bool firstRun MEMBER m_firstRun NOTIFY firstRunChanged)

    Q_PROPERTY(QString libraryState MEMBER m_libraryState NOTIFY libraryStateChanged)
    Q_PROPERTY(QVariantList games MEMBER m_games NOTIFY gamesChanged)
    Q_PROPERTY(QString gameState MEMBER m_gameState NOTIFY gameStateChanged)
    Q_PROPERTY(QVariantMap game MEMBER m_game NOTIFY gameChanged)

    Q_PROPERTY(QString addState MEMBER m_addState NOTIFY addStateChanged)
    Q_PROPERTY(QVariantMap detection MEMBER m_detection NOTIFY detectionChanged)

    Q_PROPERTY(QString friendsState MEMBER m_friendsState NOTIFY friendsStateChanged)
    Q_PROPERTY(QVariantList friends MEMBER m_friends NOTIFY friendsChanged)
    Q_PROPERTY(QVariantList requests MEMBER m_requests NOTIFY requestsChanged)
    Q_PROPERTY(QVariantList blocked MEMBER m_blocked NOTIFY blockedChanged)
    Q_PROPERTY(QVariantList lobbies MEMBER m_lobbies NOTIFY lobbiesChanged)

    Q_PROPERTY(QString settingsState MEMBER m_settingsState NOTIFY settingsStateChanged)
    Q_PROPERTY(QString logsState MEMBER m_logsState NOTIFY logsStateChanged)
    Q_PROPERTY(QVariantList logs MEMBER m_logs NOTIFY logsChanged)
    Q_PROPERTY(QVariantMap prefs MEMBER m_prefs NOTIFY prefsChanged)
    Q_PROPERTY(QString version MEMBER m_version NOTIFY versionChanged)
    Q_PROPERTY(QString updateChannel MEMBER m_updateChannel NOTIFY updateChannelChanged)
    Q_PROPERTY(QString updateState MEMBER m_updateState NOTIFY updateStateChanged)
    Q_PROPERTY(QString dataFolder MEMBER m_dataFolder NOTIFY dataFolderChanged)

    Q_PROPERTY(QString launchState MEMBER m_launchState NOTIFY launchStateChanged)
    Q_PROPERTY(QString launchError MEMBER m_launchError NOTIFY launchErrorChanged)
    Q_PROPERTY(bool reducedMotion MEMBER m_reducedMotion NOTIFY reducedMotionChanged)

    Q_PROPERTY(QString lastAction MEMBER m_lastAction NOTIFY lastActionChanged)
    Q_PROPERTY(QString copiedText MEMBER m_copiedText NOTIFY copiedTextChanged)
    Q_PROPERTY(QString pendingLobbyId MEMBER m_pendingLobbyId NOTIFY pendingLobbyIdChanged)

public:
    explicit CoreBridge(QObject *parent = nullptr);
    ~CoreBridge() override;

    bool isFake() const;
    quint32 abiVersion() const;
    QString lastError() const;

    Q_INVOKABLE void shutdown();
    Q_INVOKABLE bool createCore(quint32 abiVersion, bool fake = false);
    Q_INVOKABLE bool testConnection(const QString &server);
    Q_INVOKABLE QVariantMap login(const QString &server, const QString &username,
                                   const QString &password);
    Q_INVOKABLE bool signIn(const QString &host, const QString &username,
                            const QString &password, bool remember);
    Q_INVOKABLE bool signOut();

    Q_INVOKABLE void listGames();
    Q_INVOKABLE void getGame(const QString &id);
    Q_INVOKABLE bool setFavorite(const QString &id, bool value);
    Q_INVOKABLE bool playGame(const QString &id);
    Q_INVOKABLE bool repairGame(const QString &id);
    Q_INVOKABLE bool restoreGame(const QString &id);
    Q_INVOKABLE bool removeGame(const QString &id);
    Q_INVOKABLE bool saveGameOptions(const QString &id, const QString &args,
                                     const QString &preload);

    Q_INVOKABLE void chooseExecutable();
    Q_INVOKABLE void setChosenExecutable(const QString &path);
    Q_INVOKABLE QVariantMap detectGame(const QString &path);
    Q_INVOKABLE bool installGame(const QVariantMap &review);
    Q_INVOKABLE void cancelInstall();
    Q_INVOKABLE bool rollbackInstall();

    Q_INVOKABLE QString getPreference(const QString &key, const QString &fallback) const;
    Q_INVOKABLE bool setPreference(const QString &key, const QString &value);
    Q_INVOKABLE bool setUpdateChannel(const QString &channel);
    Q_INVOKABLE bool checkForUpdates();
    Q_INVOKABLE bool openDataFolder();
    Q_INVOKABLE bool copyLog();
    Q_INVOKABLE bool copyDiagnostics();
    Q_INVOKABLE bool setReducedMotion(bool value);
    Q_INVOKABLE bool handleDeepLink(const QString &url);

    Q_INVOKABLE bool revokeToken();
    Q_INVOKABLE bool listFriends();
    Q_INVOKABLE bool addFriend(const QString &nameOrId);
    Q_INVOKABLE bool removeFriend(const QString &id);
    Q_INVOKABLE bool blockFriend(const QString &id);
    Q_INVOKABLE bool unblockFriend(const QString &id);
    Q_INVOKABLE bool acceptRequest(const QString &id);
    Q_INVOKABLE bool declineRequest(const QString &id);
    Q_INVOKABLE bool cancelRequest(const QString &id);
    Q_INVOKABLE bool listLobbies();
    Q_INVOKABLE bool joinLobby(const QString &id);
    Q_INVOKABLE bool inviteFriend(const QString &id, const QString &lobbyId);

    Q_INVOKABLE QVariantMap installLibrary(const QString &path);
    Q_INVOKABLE QString restoreLibrary(const QString &path);
    Q_INVOKABLE QVariantMap configureGame(const QString &path, quint32 appid);
    Q_INVOKABLE quint32 launchGame(const QString &path);
    Q_INVOKABLE QString unpackSteamStub(const QString &path);
    Q_INVOKABLE QVariantList libraryList();
    Q_INVOKABLE QVariantMap libraryAdd(const QString &path);
    Q_INVOKABLE bool libraryRemove(const QString &path);

signals:
    void operationFinished(const QString &operation, bool ok, const QString &message);
    void pickExecutableRequested();
    void progress(const QString &step, const QString &message);
    void errorOccurred(quint32 status, const QString &message);
    void errorChanged();

    void connectionStateChanged();
    void serverAddressChanged();
    void accountNameChanged();
    void rememberSessionChanged();
    void firstRunChanged();
    void libraryStateChanged();
    void gamesChanged();
    void gameStateChanged();
    void gameChanged();
    void addStateChanged();
    void detectionChanged();
    void friendsStateChanged();
    void friendsChanged();
    void requestsChanged();
    void blockedChanged();
    void lobbiesChanged();
    void settingsStateChanged();
    void logsStateChanged();
    void logsChanged();
    void prefsChanged();
    void versionChanged();
    void updateChannelChanged();
    void updateStateChanged();
    void dataFolderChanged();
    void launchStateChanged();
    void launchErrorChanged();
    void reducedMotionChanged();
    void lastActionChanged();
    void copiedTextChanged();
    void pendingLobbyIdChanged();

private:
    bool recordStatus(quint32 status, const QString &action);
    void setLastError(const QString &message);
    void destroyCore();
    static void handleEvent(void *userData, const char *step, const char *message);

    bool finish(const QString &operation, bool ok, const QString &message);
    void log(const QString &level, const QString &message);
    void refreshState();
    QVariantList readLibrary();
    void applyLibrary(const QVariantList &records);
    bool forGame(const QString &id, const QString &operation,
                 const std::function<bool(const QString &)> &action);
    static QString joinArgs(const QVariant &args);
    static QStringList splitArgs(const QString &args);
    static quint32 toAppid(const QVariant &value, bool *ok);
    static quint64 toId(const QString &value, bool *ok);

    SapieCore *m_core = nullptr;
    QString m_lastError;

    QString m_connectionState = QStringLiteral("signedOut");
    QString m_serverAddress;
    QString m_accountName;
    bool m_rememberSession = false;
    bool m_firstRun = true;

    QString m_libraryState = QStringLiteral("loading");
    QVariantList m_games;
    QString m_gameState = QStringLiteral("empty");
    QVariantMap m_game;

    QString m_addState = QStringLiteral("idle");
    QVariantMap m_detection;

    QString m_friendsState = QStringLiteral("loading");
    QVariantList m_friends;
    QVariantList m_requests;
    QVariantList m_blocked;
    QVariantList m_lobbies;

    QString m_settingsState = QStringLiteral("loading");
    QString m_logsState = QStringLiteral("ready");
    QVariantList m_logs;
    QVariantMap m_prefs;
    QString m_version;
    QString m_updateChannel = QStringLiteral("stable");
    QString m_updateState = QStringLiteral("idle");
    QString m_dataFolder;

    QString m_launchState = QStringLiteral("idle");
    QString m_launchError;
    bool m_reducedMotion = false;

    QString m_lastAction;
    QString m_copiedText;
    QString m_pendingInstallPath;
    QString m_pendingLobbyId;
    QString m_pendingInvite;
};
