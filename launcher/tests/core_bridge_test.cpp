#include "core_bridge_test.h"

#include "CoreBridge.h"
#include "FakePlatform.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QHostAddress>
#include <QSignalSpy>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QTest>
#include <QThread>

#include <filesystem>

namespace {
void clearFakeState()
{
    const QString path = QDir::tempPath()
            + QStringLiteral("/sapie-fake-core-%1/launcher-state.json")
                      .arg(QCoreApplication::applicationPid());
    QFile::remove(path);
}

bool writeBytes(const QString &path, const QByteArray &bytes)
{
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly)) {
        return false;
    }
    return file.write(bytes) == bytes.size();
}

// The core keys the library by canonical path; Windows adds a \\?\ prefix.
QString canonicalExe(const QString &path)
{
    return QString::fromWCharArray(
        std::filesystem::canonical(path.toStdWString()).wstring().c_str());
}

struct FakeDllSource {
    QString path;
    ~FakeDllSource()
    {
        QFile::remove(path);
    }
};

struct ThreadStopper {
    QThread &thread;
    LoginServer &server;
    ~ThreadStopper()
    {
        QMetaObject::invokeMethod(&server, "stop", Qt::BlockingQueuedConnection);
        thread.quit();
        thread.wait(3000);
    }
};
} // namespace

QVector<QByteArray> LoginServer::requests() const
{
    QMutexLocker locker(&m_mutex);
    return m_requests;
}

void LoginServer::start()
{
    if (!m_server.listen(QHostAddress::LocalHost, 0)) {
        emit portReady(0);
        return;
    }
    connect(&m_server, &QTcpServer::newConnection, this, &LoginServer::onNewConnection);
    emit portReady(m_server.serverPort());
}

void LoginServer::stop()
{
    m_server.close();
    for (QTcpSocket *socket : m_sockets) {
        delete socket;
    }
    m_sockets.clear();
    QCoreApplication::processEvents();
}

void LoginServer::onNewConnection()
{
    while (m_server.hasPendingConnections()) {
        QTcpSocket *socket = m_server.nextPendingConnection();
        m_sockets.append(socket);
        connect(socket, &QTcpSocket::readyRead, socket, [this, socket]() {
            if (socket->property("sapieResponded").toBool()) {
                return;
            }
            const QByteArray request = socket->readAll();
            if (request.isEmpty()) {
                return;
            }
            socket->setProperty("sapieResponded", true);
            {
                QMutexLocker locker(&m_mutex);
                m_requests.append(request);
            }

            const QByteArray requestLine = request.split('\n').value(0).trimmed();
            const QList<QByteArray> parts = requestLine.split(' ');
            const QByteArray method = parts.value(0);
            const QByteArray path = parts.value(1);
            auto respond = [socket](int code, const QByteArray &body) {
                const char *text = code == 200 ? "OK" : code == 201 ? "Created"
                        : code == 202 ? "Accepted" : code == 204 ? "No Content"
                        : code == 401 ? "Unauthorized" : "Not Found";
                const QByteArray response = QByteArray("HTTP/1.1 ")
                        + QByteArray::number(code) + " " + text + "\r\n"
                        + "Content-Type: application/json\r\n"
                        + "Content-Length: " + QByteArray::number(body.size()) + "\r\n"
                        + "Connection: close\r\n\r\n" + body;
                socket->write(response);
                socket->flush();
                socket->disconnectFromHost();
            };

            if (path == "/api/v1/config") {
                respond(200, R"({"allow_register":false})");
                return;
            }
            if (path == "/api/v1/login") {
                respond(200, R"({"session_token":"tok-123","steam_id":76561197960265731,"persona_name":"Tester"})");
                return;
            }
            if (!request.contains("Authorization: Bearer tok-123")) {
                respond(401, "unauthorized");
                return;
            }

            if (path == "/api/v1/friends" && method == "GET") {
                respond(200, R"({"friends":[{"steam_id":76561197960265732,"persona_name":"Bob","persona_state":1,"appid":480,"rich_presence":{},"lobby_id":0,"connect_str":""}]})");
            } else if (path == "/api/v1/friends/requests" && method == "GET") {
                respond(200, R"({"incoming":[{"id":7,"from_steam_id":76561197960265732,"to_steam_id":76561197960265731,"persona_name":"Bob","message":"play?","created_at":1,"status":"pending"}],"outgoing":[{"id":9,"from_steam_id":76561197960265731,"to_steam_id":76561197960265733,"persona_name":"Carol","message":"hello","created_at":2,"status":"pending"}]})");
            } else if (path == "/api/v1/friends/requests" && method == "POST") {
                respond(201, R"({"id":10,"status":"pending","created_at":3})");
            } else if (path.startsWith("/api/v1/friends/requests/") && method == "POST") {
                respond(200, R"({"status":"ok"})");
            } else if (path.startsWith("/api/v1/friends/requests/") && method == "DELETE") {
                respond(204, {});
            } else if (path == "/api/v1/friends/blocked" && method == "GET") {
                respond(200, R"({"blocked":[{"steam_id":76561197960265734,"persona_name":"Dave","created_at":4}]})");
            } else if (path.startsWith("/api/v1/friends/") && path.endsWith("/invite")
                       && method == "POST") {
                respond(202, {});
            } else if (path.startsWith("/api/v1/friends/") && path.endsWith("/block")) {
                respond(204, {});
            } else if (path.startsWith("/api/v1/friends/") && method == "DELETE") {
                respond(204, {});
            } else if (path == "/api/v1/lobbies" && method == "GET") {
                respond(200, R"({"lobbies":[{"lobby_id":55,"owner":76561197960265732,"appid":480,"member_limit":4,"type":2,"joinable":true,"members":[76561197960265732]}]})");
            } else if (path.startsWith("/api/v1/lobbies/") && path.endsWith("/invite")
                       && method == "POST") {
                respond(200, R"({"url":"sapie://join/55?exp=1&sig=abc","expires_at":1})");
            } else if (path.startsWith("/api/v1/lobbies/") && path.endsWith("/join")
                       && method == "POST") {
                respond(200, R"({"lobby_id":55})");
            } else if (path == "/api/v1/logout" && method == "POST") {
                respond(204, {});
            } else {
                respond(404, "not found");
            }
        });
    }
}

void CoreBridgeTest::init()
{
    clearFakeState();
}

void CoreBridgeTest::unknownAbiVersionHasClearError()
{
    CoreBridge bridge;
    QVERIFY(!bridge.createCore(bridge.abiVersion() + 1, true));
    QVERIFY2(bridge.lastError().contains("ABI", Qt::CaseInsensitive),
             qPrintable(bridge.lastError()));
}

void CoreBridgeTest::fakeCoreDrivesLoginDetectConfigureAndLaunch()
{
    LoginServer loginServer;
    QThread serverThread;
    loginServer.moveToThread(&serverThread);

    QSignalSpy portSpy(&loginServer, &LoginServer::portReady);
    connect(&serverThread, &QThread::started, &loginServer, &LoginServer::start);
    serverThread.start();
    ThreadStopper stop{serverThread, loginServer};
    QVERIFY(portSpy.wait(3000));
    QCOMPARE(portSpy.count(), 1);
    const quint16 port = portSpy.first().at(0).toUInt();
    QVERIFY(port != 0);

    QTemporaryDir gameDir;
    QVERIFY(gameDir.isValid());
    const QString rawExe = gameDir.filePath(QStringLiteral("game.exe"));
    QByteArray image(0x100, '\0');
    image[0] = 'M';
    image[1] = 'Z';
    image[0x3c] = static_cast<char>(0x80);
    image[0x80] = 'P';
    image[0x81] = 'E';
    image[0x84] = static_cast<char>(0x64);
    image[0x85] = static_cast<char>(0x86);
    QVERIFY(writeBytes(rawExe, image));
    const QString exe = canonicalExe(rawExe);

    QByteArray dll("SteamUser019", 12);
    dll.append('\0');
    dll.append("SteamClient017", 14);
    dll.append('\0');
    const QString backup = gameDir.filePath(QStringLiteral("steam_api64.dll.sapie-original.bak"));
    QVERIFY(writeBytes(backup, dll));

    CoreBridge bridge;
    QSignalSpy progressSpy(&bridge, &CoreBridge::progress);
    QVERIFY(bridge.createCore(bridge.abiVersion(), true));

    const QVariantMap session = bridge.login(QStringLiteral("127.0.0.1:%1").arg(port),
                                             QStringLiteral("tester"), QStringLiteral("pw"));
    QVERIFY(!session.isEmpty());
    QCOMPARE(session.value(QStringLiteral("steamId")).toULongLong(),
             QVariant(76561197960265731ULL).toULongLong());
    QCOMPARE(session.value(QStringLiteral("personaName")).toString(), QStringLiteral("Tester"));

    const QVariantMap detected = bridge.detectGame(exe);
    QVERIFY(!detected.isEmpty());
    QCOMPARE(detected.value(QStringLiteral("arch")).toString(), QStringLiteral("x64"));
    QCOMPARE(detected.value(QStringLiteral("era")).toString(), QStringLiteral("modern"));
    QVERIFY(!progressSpy.isEmpty());

    const QVariantMap record = bridge.libraryAdd(exe);
    QVERIFY(!record.isEmpty());

    const QVariantMap configured = bridge.configureGame(exe, 480);
    QVERIFY(!configured.isEmpty());
    QCOMPARE(configured.value(QStringLiteral("gameServer")).toString(),
             QStringLiteral("127.0.0.1:47584"));

    QCOMPARE(bridge.launchGame(exe), 4242U);
    QVERIFY(bridge.libraryRemove(exe));
}

void CoreBridgeTest::fakeCoreDrivesScreenMethods()
{
#if defined(Q_OS_WIN)
    const QString dllSuffix = QStringLiteral("windows-modern-x64");
    const QString dllName = QStringLiteral("steam_api64.dll");
#else
    const QString dllSuffix = QStringLiteral("linux-modern-x64");
    const QString dllName = QStringLiteral("libsteam_api.so");
#endif
    const QString sourceDir = QCoreApplication::applicationDirPath()
            + QStringLiteral("/resources/steam_api/") + dllSuffix;
    QVERIFY(QDir().mkpath(sourceDir));
    const QString sourceDll = sourceDir + QLatin1Char('/') + dllName;
    QVERIFY(writeBytes(sourceDll, QByteArray("fake sapie library")));
    FakeDllSource sourceGuard{sourceDll};

    LoginServer loginServer;
    QThread serverThread;
    loginServer.moveToThread(&serverThread);
    QSignalSpy portSpy(&loginServer, &LoginServer::portReady);
    connect(&serverThread, &QThread::started, &loginServer, &LoginServer::start);
    serverThread.start();
    ThreadStopper stop{serverThread, loginServer};
    QVERIFY(portSpy.wait(3000));
    const quint16 port = portSpy.first().at(0).toUInt();
    QVERIFY(port != 0);

    QTemporaryDir gameDir;
    QVERIFY(gameDir.isValid());
    const QString rawExe = gameDir.filePath(QStringLiteral("game.exe"));
    QByteArray image(0x100, '\0');
    image[0] = 'M';
    image[1] = 'Z';
    image[0x3c] = static_cast<char>(0x80);
    image[0x80] = 'P';
    image[0x81] = 'E';
    image[0x84] = static_cast<char>(0x64);
    image[0x85] = static_cast<char>(0x86);
    QVERIFY(writeBytes(rawExe, image));
    const QString exe = canonicalExe(rawExe);
    const QString backup = gameDir.filePath(QStringLiteral("steam_api64.dll.sapie-original.bak"));
    QByteArray dll("SteamUser019", 12);
    dll.append('\0');
    dll.append("SteamClient017", 14);
    dll.append('\0');
    QVERIFY(writeBytes(backup, dll));

    CoreBridge bridge;
    QSignalSpy operationSpy(&bridge, &CoreBridge::operationFinished);
    QSignalSpy pickerSpy(&bridge, &CoreBridge::pickExecutableRequested);
    QVERIFY(bridge.createCore(bridge.abiVersion(), true));
    QCOMPARE(bridge.isFake(), false);
    QCOMPARE(bridge.property("firstRun").toBool(), true);
    QCOMPARE(bridge.property("settingsState").toString(),
             QStringLiteral("ready"));
    QVERIFY(!bridge.property("version").toString().isEmpty());
    QVERIFY(!bridge.property("dataFolder").toString().isEmpty());

    QVERIFY(bridge.signIn(QStringLiteral("127.0.0.1:%1").arg(port),
                          QStringLiteral("tester"), QStringLiteral("pw"), true));
    QCOMPARE(bridge.property("connectionState").toString(),
             QStringLiteral("signedIn"));
    QCOMPARE(bridge.property("accountName").toString(),
             QStringLiteral("Tester"));
    QCOMPARE(bridge.property("rememberSession").toBool(), true);
    QCOMPARE(bridge.property("firstRun").toBool(), false);

    QVERIFY(!bridge.libraryAdd(exe).isEmpty());
    bridge.cancelInstall();
    QCOMPARE(bridge.property("addState").toString(), QStringLiteral("idle"));
    QVERIFY(bridge.rollbackInstall());
    QCOMPARE(bridge.property("addState").toString(), QStringLiteral("idle"));
    QVERIFY(bridge.libraryRemove(exe));

    bridge.chooseExecutable();
    QCOMPARE(pickerSpy.count(), 1);
    bridge.setChosenExecutable(exe);
    QCOMPARE(bridge.property("addState").toString(),
             QStringLiteral("detecting"));
    const QVariantMap detected = bridge.detectGame(exe);
    QCOMPARE(detected.value(QStringLiteral("arch")).toString(), QStringLiteral("x64"));
    QCOMPARE(bridge.property("addState").toString(), QStringLiteral("review"));
    QCOMPARE(bridge.property("detection").toMap()
             .value(QStringLiteral("interfaces")).toInt(), 2);

    const QVariantMap review{{QStringLiteral("appid"), QStringLiteral("480")},
                             {QStringLiteral("args"), QStringLiteral("-windowed -nopause")},
                             {QStringLiteral("preload"), QString()},
                             {QStringLiteral("unpackStub"), false}};
    QVERIFY2(bridge.installGame(review), qPrintable(bridge.lastError()));
    QCOMPARE(bridge.property("addState").toString(), QStringLiteral("done"));
    QVERIFY(!bridge.installGame(review));
    QCOMPARE(bridge.property("addState").toString(), QStringLiteral("duplicate"));

    bridge.listGames();
    const QVariantList games = bridge.property("games").toList();
    QCOMPARE(games.size(), 1);
    const QVariantMap game = games.first().toMap();
    QCOMPARE(game.value(QStringLiteral("id")).toString(), exe);
    QCOMPARE(game.value(QStringLiteral("name")).toString(), QStringLiteral("game"));
    QCOMPARE(game.value(QStringLiteral("appid")).toUInt(), 480U);
    QCOMPARE(game.value(QStringLiteral("installed")).toBool(), true);
    QCOMPARE(game.value(QStringLiteral("missing")).toBool(), false);
    QCOMPARE(game.value(QStringLiteral("playtimeSeconds")).toInt(), -1);
    QCOMPARE(game.value(QStringLiteral("cover")).toString(), QString());

    bridge.getGame(exe);
    QCOMPARE(bridge.property("gameState").toString(), QStringLiteral("ready"));
    QVERIFY(bridge.setFavorite(exe, true));
    bridge.getGame(exe);
    QVERIFY(bridge.property("game").toMap()
            .value(QStringLiteral("favorite")).toBool());
    QVERIFY(bridge.saveGameOptions(exe, QStringLiteral("-windowed -nopause"), QString()));
    bridge.getGame(exe);
    QCOMPARE(bridge.property("game").toMap()
             .value(QStringLiteral("arguments")).toString(),
             QStringLiteral("-windowed -nopause"));
    QVERIFY(bridge.playGame(exe));
    QCOMPARE(bridge.property("launchState").toString(),
             QStringLiteral("running"));
    QVERIFY(bridge.repairGame(exe));
    QVERIFY(bridge.restoreGame(exe));

    QVERIFY(bridge.setPreference(QStringLiteral("librarySort"), QStringLiteral("Last played")));
    QCOMPARE(bridge.getPreference(QStringLiteral("librarySort"), QStringLiteral("Name")),
             QStringLiteral("Last played"));
    QVERIFY(bridge.setUpdateChannel(QStringLiteral("beta")));
    QCOMPARE(bridge.property("updateChannel").toString(),
             QStringLiteral("beta"));
    QVERIFY(bridge.checkForUpdates());
    QCOMPARE(bridge.property("updateState").toString(),
             QStringLiteral("current"));
    QVERIFY(bridge.setReducedMotion(true));
    QVERIFY(bridge.property("reducedMotion").toBool());

    QVERIFY(bridge.copyLog());
    QVERIFY(!bridge.property("copiedText").toString().contains(
            QStringLiteral("pw")));
    QVERIFY(bridge.copyDiagnostics());
    QVERIFY(!bridge.property("copiedText").toString().contains(
            QStringLiteral("pw")));
    bridge.openDataFolder();
    QCOMPARE(bridge.property("lastAction").toString(),
             QStringLiteral("openDataFolder"));

    QVERIFY(bridge.handleDeepLink(QStringLiteral("sapie://join/123?exp=1&sig=abc")));
    QCOMPARE(bridge.property("pendingLobbyId").toString(),
             QStringLiteral("123"));
    QVERIFY(!bridge.handleDeepLink(QStringLiteral("sapie://join/not-a-number")));

    QVERIFY(bridge.removeGame(exe));
    QVERIFY(bridge.signOut());
    QCOMPARE(bridge.property("connectionState").toString(),
             QStringLiteral("signedOut"));
    QVERIFY(operationSpy.count() >= 20);
}

void CoreBridgeTest::fakeCoreDrivesServerSocialMethods()
{
    LoginServer loginServer;
    QThread serverThread;
    loginServer.moveToThread(&serverThread);
    QSignalSpy portSpy(&loginServer, &LoginServer::portReady);
    connect(&serverThread, &QThread::started, &loginServer, &LoginServer::start);
    serverThread.start();
    ThreadStopper stop{serverThread, loginServer};
    QVERIFY(portSpy.wait(3000));
    const quint16 port = portSpy.first().at(0).toUInt();
    QVERIFY(port != 0);

    CoreBridge bridge;
    QSignalSpy operationSpy(&bridge, &CoreBridge::operationFinished);
    QVERIFY(bridge.createCore(bridge.abiVersion(), true));
    QVERIFY(bridge.signIn(QStringLiteral("127.0.0.1:%1").arg(port),
                          QStringLiteral("tester"), QStringLiteral("pw"), true));
    QCOMPARE(bridge.property("connectionState").toString(), QStringLiteral("signedIn"));

    QVERIFY(bridge.listFriends());
    QCOMPARE(bridge.property("friendsState").toString(), QStringLiteral("ready"));
    const QVariantList friends = bridge.property("friends").toList();
    QCOMPARE(friends.size(), 1);
    const QVariantMap friendRecord = friends.first().toMap();
    QCOMPARE(friendRecord.value(QStringLiteral("id")).toString(),
             QStringLiteral("76561197960265732"));
    QCOMPARE(friendRecord.value(QStringLiteral("name")).toString(), QStringLiteral("Bob"));
    QCOMPARE(friendRecord.value(QStringLiteral("status")).toString(), QStringLiteral("online"));

    const QVariantList requests = bridge.property("requests").toList();
    QCOMPARE(requests.size(), 2);
    QCOMPARE(requests.first().toMap().value(QStringLiteral("id")).toString(),
             QStringLiteral("7"));
    QCOMPARE(requests.first().toMap().value(QStringLiteral("direction")).toString(),
             QStringLiteral("incoming"));
    QCOMPARE(requests.last().toMap().value(QStringLiteral("direction")).toString(),
             QStringLiteral("outgoing"));
    QCOMPARE(bridge.property("blocked").toList().size(), 1);
    const QVariantList lobbies = bridge.property("lobbies").toList();
    QCOMPARE(lobbies.size(), 1);
    const QVariantMap lobby = lobbies.first().toMap();
    QCOMPARE(lobby.value(QStringLiteral("id")).toString(), QStringLiteral("55"));
    QCOMPARE(lobby.value(QStringLiteral("members")).toInt(), 1);
    QCOMPARE(lobby.value(QStringLiteral("capacity")).toInt(), 4);
    QCOMPARE(lobby.value(QStringLiteral("lobbyState")).toString(),
             QStringLiteral("joinable"));

    const QString target = QStringLiteral("76561197960265732");
    QVERIFY(bridge.addFriend(QStringLiteral("bob")));
    QVERIFY(bridge.acceptRequest(QStringLiteral("7")));
    QVERIFY(bridge.declineRequest(QStringLiteral("8")));
    QVERIFY(bridge.cancelRequest(QStringLiteral("9")));
    QVERIFY(bridge.removeFriend(target));
    QVERIFY(bridge.blockFriend(target));
    QVERIFY(bridge.unblockFriend(target));
    QVERIFY(bridge.listLobbies());
    QVERIFY(bridge.handleDeepLink(QStringLiteral("sapie://join/55?exp=1&sig=abc")));
    QVERIFY(bridge.joinLobby(QStringLiteral("55")));
    QVERIFY(bridge.inviteFriend(target, QString()));
    QVERIFY(bridge.inviteFriend(target, QStringLiteral("55")));
    QVERIFY(!bridge.joinLobby(QStringLiteral("not-a-number")));

    QVERIFY(bridge.revokeToken());
    QCOMPARE(bridge.property("connectionState").toString(), QStringLiteral("signedOut"));
    QCOMPARE(bridge.property("friendsState").toString(), QStringLiteral("loading"));

    const QVector<QByteArray> requestsMade = loginServer.requests();
    bool sawBearer = false;
    for (const QByteArray &request : requestsMade) {
        if (request.startsWith("POST /api/v1/login")) {
            QVERIFY(!request.contains("Authorization:"));
            continue;
        }
        sawBearer = sawBearer || request.contains("Authorization: Bearer tok-123");
        QVERIFY(!request.toLower().contains("password"));
        QVERIFY(!request.contains("\"steam_id\""));
    }
    QVERIFY(sawBearer);
    QVERIFY(operationSpy.count() >= 16);
}

void CoreBridgeTest::fakePlatformRecordsNativeCalls()
{
    FakePlatform platform;
    platform.executable = QStringLiteral("/games/game.exe");
    QCOMPARE(platform.chooseExecutable(nullptr), QStringLiteral("/games/game.exe"));
    platform.openFolder(QStringLiteral("/games"));
    QVERIFY(platform.registerProtocolHandler());
    QVERIFY(platform.enableSingleInstance());
    platform.showTrayMessage(QStringLiteral("SAPIE"), QStringLiteral("Ready"));
    QCOMPARE(platform.openedFolders.size(), 1);
    QCOMPARE(platform.trayMessages.size(), 1);
    QVERIFY(platform.protocolRegistered);
    QVERIFY(platform.singleInstance);
}

QTEST_MAIN(CoreBridgeTest)
