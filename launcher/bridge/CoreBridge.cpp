#include "CoreBridge.h"

extern "C" {
#include "sapie_core.h"
}

#include <QClipboard>
#include <QDateTime>
#include <QDesktopServices>
#include <QFileInfo>
#include <QGuiApplication>
#include <QStringList>
#include <QUrl>

#include <limits>
#include <vector>

namespace {
const char *statusMessage(SapieStatus status)
{
    switch (status) {
    case SAPIE_OK:
        return "OK";
    case SAPIE_ERR_INVALID_ARGUMENT:
        return "The core received an invalid argument.";
    case SAPIE_ERR_INVALID_PATH:
        return "The path is empty, missing or outside the game directory.";
    case SAPIE_ERR_NOT_FOUND:
        return "The game, library or interface data was not found.";
    case SAPIE_ERR_PERMISSION_DENIED:
        return "Permission was denied.";
    case SAPIE_ERR_BACKUP_CONFLICT:
        return "An existing backup does not belong to SAPIE.";
    case SAPIE_ERR_INVALID_SERVER:
        return "The server address is not valid.";
    case SAPIE_ERR_UNREACHABLE:
        return "The SAPIE server could not be reached.";
    case SAPIE_ERR_BAD_CREDENTIALS:
        return "The username or password is wrong.";
    case SAPIE_ERR_SERVER_TOO_OLD:
        return "The server is too old for this launcher.";
    case SAPIE_ERR_SERVER:
        return "The server returned an error.";
    case SAPIE_ERR_TOKEN_EXPIRED:
        return "The session has ended. Sign in again.";
    case SAPIE_ERR_KEYCHAIN:
        return "The operating system keychain could not be used.";
    case SAPIE_ERR_IO:
        return "A local file could not be read or written.";
    case SAPIE_ERR_PROCESS:
        return "The external process could not be started or read.";
    case SAPIE_ERR_ABI_VERSION:
        return "This core does not support that ABI version.";
    }
    return "The core returned an unknown status.";
}

QString connectionStateFor(SapieStatus status)
{
    switch (status) {
    case SAPIE_ERR_INVALID_SERVER:
    case SAPIE_ERR_INVALID_ARGUMENT:
        return QStringLiteral("badHost");
    case SAPIE_ERR_UNREACHABLE:
    case SAPIE_ERR_SERVER:
    case SAPIE_ERR_IO:
        return QStringLiteral("unreachable");
    case SAPIE_ERR_BAD_CREDENTIALS:
    case SAPIE_ERR_TOKEN_EXPIRED:
        return QStringLiteral("unauthorized");
    case SAPIE_ERR_SERVER_TOO_OLD:
        return QStringLiteral("serverTooOld");
    default:
        return QStringLiteral("signedOut");
    }
}

QString fromChar(const char *value)
{
    return value == nullptr ? QString() : QString::fromUtf8(value);
}

QStringList fromArray(char **values, size_t count)
{
    QStringList result;
    if (values == nullptr) {
        return result;
    }
    for (size_t i = 0; i < count; ++i) {
        result.append(fromChar(values[i]));
    }
    return result;
}

struct CharOwner {
    char *value = nullptr;
    ~CharOwner() { sapie_string_free(value); }
};

struct SessionOwner {
    SapieSessionInfo *value = nullptr;
    ~SessionOwner() { sapie_session_info_free(value); }
};

struct DetectOwner {
    SapieDetectResult *value = nullptr;
    ~DetectOwner() { sapie_detect_result_free(value); }
};

struct InstallOwner {
    SapieInstallResult *value = nullptr;
    ~InstallOwner() { sapie_install_result_free(value); }
};

struct ConfigureOwner {
    SapieConfigureResult *value = nullptr;
    ~ConfigureOwner() { sapie_configure_result_free(value); }
};

struct RecordOwner {
    SapieGameRecord *value = nullptr;
    ~RecordOwner() { sapie_game_record_free(value); }
};

struct RecordArrayOwner {
    SapieGameRecord *value = nullptr;
    size_t count = 0;
    ~RecordArrayOwner() { sapie_game_record_array_free(value, count); }
};

QVariantMap recordToMap(const SapieGameRecord &record)
{
    QVariantMap result;
    result["exePath"] = fromChar(record.exe_path);
    result["arch"] = fromChar(record.arch);
    result["era"] = fromChar(record.era);
    result["interfaces"] = fromArray(record.interfaces, record.interface_count);
    result["originalLibrary"] = fromChar(record.original_library);
    result["library"] = fromChar(record.library);
    result["appid"] = record.appid;
    result["configured"] = record.configured != 0;
    result["launchArgs"] = fromArray(record.launch_args, record.launch_arg_count);
    result["ldPreload"] = fromChar(record.ld_preload);
    result["lastPlayed"] = static_cast<qulonglong>(record.last_played);
    result["favorite"] = record.favorite != 0;
    return result;
}

QVariantMap screenRecord(const QVariantMap &raw)
{
    const QString path = raw.value(QStringLiteral("exePath")).toString();
    const qulonglong lastPlayed = raw.value(QStringLiteral("lastPlayed")).toULongLong();
    QVariantMap result;
    result["id"] = path;
    result["path"] = path;
    result["name"] = QFileInfo(path).completeBaseName();
    result["appid"] = raw.value(QStringLiteral("appid")).toUInt();
    result["cover"] = QString();
    result["installed"] = !raw.value(QStringLiteral("library")).toString().isEmpty();
    result["missing"] = !QFileInfo::exists(path);
    result["favorite"] = raw.value(QStringLiteral("favorite")).toBool();
    result["arch"] = raw.value(QStringLiteral("arch")).toString();
    result["era"] = raw.value(QStringLiteral("era")).toString();
    result["interfaces"] = raw.value(QStringLiteral("interfaces")).toList().size();
    result["backup"] = raw.value(QStringLiteral("originalLibrary")).toString();
    result["arguments"] = raw.value(QStringLiteral("launchArgs")).toStringList().join(' ');
    result["preload"] = raw.value(QStringLiteral("ldPreload")).toString();
    result["achievements"] = QVariantList{};
    result["recentLaunches"] = QVariantList{};
    result["lastPlayed"] = lastPlayed == 0
            ? QString()
            : QDateTime::fromSecsSinceEpoch(static_cast<qint64>(lastPlayed)).toString(Qt::ISODate);
    result["playtimeSeconds"] = -1;
    return result;
}
} // namespace

CoreBridge::CoreBridge(QObject *parent)
    : QObject(parent)
{
}

CoreBridge::~CoreBridge()
{
    destroyCore();
}

bool CoreBridge::isFake() const
{
    return false;
}

quint32 CoreBridge::abiVersion() const
{
    return sapie_core_abi_version();
}

QString CoreBridge::lastError() const
{
    return m_lastError;
}

void CoreBridge::shutdown()
{
    destroyCore();
}

bool CoreBridge::createCore(quint32 abiVersion, bool fake)
{
    destroyCore();
    const SapieStatus status = fake
            ? sapie_core_create_fake(abiVersion, &CoreBridge::handleEvent, this, &m_core)
            : sapie_core_create(abiVersion, &CoreBridge::handleEvent, this, &m_core);
    if (!recordStatus(status, QStringLiteral("Create core"))) {
        return false;
    }
    if (m_core == nullptr) {
        setLastError(QStringLiteral("Create core: the core returned no handle."));
        emit errorOccurred(SAPIE_ERR_IO, m_lastError);
        return false;
    }
    refreshState();
    return true;
}

bool CoreBridge::testConnection(const QString &server)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return false;
    }
    return recordStatus(sapie_core_test_connection(m_core, qUtf8Printable(server)),
                        QStringLiteral("Test connection"));
}

QVariantMap CoreBridge::login(const QString &server, const QString &username,
                              const QString &password)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return {};
    }
    SapieSessionInfo *rawSession = nullptr;
    const SapieStatus status = sapie_core_login(m_core, qUtf8Printable(server),
                                                qUtf8Printable(username),
                                                qUtf8Printable(password), &rawSession);
    SessionOwner session{rawSession};
    if (!recordStatus(status, QStringLiteral("Sign in"))) {
        return {};
    }

    QVariantMap result;
    result["steamId"] = static_cast<qulonglong>(session.value->steam_id);
    result["personaName"] = fromChar(session.value->persona_name);
    result["server"] = fromChar(session.value->server);
    result["expiresAt"] = static_cast<qulonglong>(session.value->expires_at);
    return result;
}

bool CoreBridge::signIn(const QString &host, const QString &username,
                        const QString &password, bool remember)
{
    if (host.trimmed().isEmpty() || username.trimmed().isEmpty() || password.isEmpty()) {
        m_connectionState = QStringLiteral("badHost");
        emit connectionStateChanged();
        return finish(QStringLiteral("signIn"), false,
                      QStringLiteral("Server, username and password are required."));
    }
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return finish(QStringLiteral("signIn"), false, m_lastError);
    }

    m_connectionState = QStringLiteral("checking");
    emit connectionStateChanged();
    SapieStatus status = sapie_core_test_connection(m_core, qUtf8Printable(host));
    if (!recordStatus(status, QStringLiteral("Test connection"))) {
        m_connectionState = connectionStateFor(status);
        emit connectionStateChanged();
        return finish(QStringLiteral("signIn"), false, m_lastError);
    }

    SapieSessionInfo *rawSession = nullptr;
    status = sapie_core_login(m_core, qUtf8Printable(host), qUtf8Printable(username),
                              qUtf8Printable(password), &rawSession);
    SessionOwner session{rawSession};
    if (!recordStatus(status, QStringLiteral("Sign in"))) {
        m_connectionState = connectionStateFor(status);
        emit connectionStateChanged();
        return finish(QStringLiteral("signIn"), false, m_lastError);
    }

    m_serverAddress = fromChar(session.value->server);
    m_accountName = fromChar(session.value->persona_name);
    if (m_accountName.isEmpty()) {
        m_accountName = username;
    }
    m_rememberSession = remember;
    m_firstRun = false;
    m_connectionState = QStringLiteral("signedIn");
    m_settingsState = QStringLiteral("ready");
    m_logsState = QStringLiteral("ready");
    emit serverAddressChanged();
    emit accountNameChanged();
    emit rememberSessionChanged();
    emit firstRunChanged();
    emit connectionStateChanged();
    emit settingsStateChanged();
    emit logsStateChanged();
    setPreference(QStringLiteral("server"), m_serverAddress);
    setPreference(QStringLiteral("accountName"), m_accountName);
    setPreference(QStringLiteral("rememberSession"), remember ? QStringLiteral("true")
                                                              : QStringLiteral("false"));
    log(QStringLiteral("Info"), QStringLiteral("Signed in to %1 as %2.")
                .arg(m_serverAddress, m_accountName));
    return finish(QStringLiteral("signIn"), true,
                  QStringLiteral("Signed in to %1.").arg(m_serverAddress));
}

bool CoreBridge::signOut()
{
    if (m_core != nullptr) {
        if (!recordStatus(sapie_core_sign_out(m_core), QStringLiteral("Sign out"))) {
            return finish(QStringLiteral("signOut"), false, m_lastError);
        }
    }
    m_connectionState = QStringLiteral("signedOut");
    m_accountName.clear();
    emit connectionStateChanged();
    emit accountNameChanged();
    return finish(QStringLiteral("signOut"), true,
                  QStringLiteral("Signed out. The local token was cleared."));
}

QVariantList CoreBridge::readLibrary()
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return {};
    }
    SapieGameRecord *rawRecords = nullptr;
    size_t count = 0;
    const SapieStatus status = sapie_core_library_list(m_core, &rawRecords, &count);
    RecordArrayOwner records{rawRecords, count};
    if (!recordStatus(status, QStringLiteral("List library"))) {
        return {};
    }

    QVariantList result;
    for (size_t i = 0; i < count; ++i) {
        result.append(recordToMap(records.value[i]));
    }
    return result;
}

void CoreBridge::applyLibrary(const QVariantList &records)
{
    QVariantList screens;
    for (const QVariant &raw : records) {
        screens.append(screenRecord(raw.toMap()));
    }
    m_games = screens;
    m_libraryState = screens.isEmpty() ? QStringLiteral("empty") : QStringLiteral("ready");
    emit gamesChanged();
    emit libraryStateChanged();

    const QString selectedId = m_game.value(QStringLiteral("id")).toString();
    if (selectedId.isEmpty()) {
        return;
    }
    for (const QVariant &screen : screens) {
        const QVariantMap record = screen.toMap();
        if (record.value(QStringLiteral("id")).toString() == selectedId) {
            m_game = record;
            emit gameChanged();
            return;
        }
    }
    m_game.clear();
    emit gameChanged();
}

void CoreBridge::listGames()
{
    const QVariantList records = readLibrary();
    if (!m_lastError.isEmpty()) {
        m_libraryState = QStringLiteral("error");
        emit libraryStateChanged();
        finish(QStringLiteral("listGames"), false, m_lastError);
        return;
    }
    applyLibrary(records);
    finish(QStringLiteral("listGames"), true, QStringLiteral("Library read."));
}

QVariantList CoreBridge::libraryList()
{
    return readLibrary();
}

void CoreBridge::getGame(const QString &id)
{
    for (const QVariant &entry : m_games) {
        const QVariantMap record = entry.toMap();
        if (record.value(QStringLiteral("id")).toString() != id) {
            continue;
        }
        m_game = record;
        m_gameState = record.value(QStringLiteral("missing")).toBool()
                ? QStringLiteral("missing")
                : QStringLiteral("ready");
        emit gameChanged();
        emit gameStateChanged();
        finish(QStringLiteral("getGame"), true, QStringLiteral("Game details read."));
        return;
    }
    m_game.clear();
    m_gameState = QStringLiteral("empty");
    emit gameChanged();
    emit gameStateChanged();
    finish(QStringLiteral("getGame"), false, QStringLiteral("No game is selected."));
}

bool CoreBridge::forGame(const QString &id, const QString &operation,
                         const std::function<bool(const QString &)> &action)
{
    QString path = m_game.value(QStringLiteral("id")).toString() == id
            ? m_game.value(QStringLiteral("path")).toString()
            : QString();
    if (path.isEmpty()) {
        for (const QVariant &entry : m_games) {
            const QVariantMap record = entry.toMap();
            if (record.value(QStringLiteral("id")).toString() == id) {
                path = record.value(QStringLiteral("path")).toString();
                break;
            }
        }
    }
    if (path.isEmpty()) {
        return finish(operation, false, QStringLiteral("Game is not in the library."));
    }
    const bool ok = action(path);
    return finish(operation, ok, ok ? QStringLiteral("Operation finished.")
                                    : m_lastError);
}

bool CoreBridge::setFavorite(const QString &id, bool value)
{
    return forGame(id, QStringLiteral("setFavorite"), [this, value](const QString &path) {
        if (!recordStatus(sapie_core_set_favorite(m_core, qUtf8Printable(path), value ? 1 : 0),
                          QStringLiteral("Set favorite"))) {
            return false;
        }
        applyLibrary(readLibrary());
        return true;
    });
}

bool CoreBridge::playGame(const QString &id)
{
    m_launchState = QStringLiteral("launching");
    m_launchError.clear();
    emit launchStateChanged();
    emit launchErrorChanged();
    const bool ok = forGame(id, QStringLiteral("playGame"), [this](const QString &path) {
        return launchGame(path) != 0;
    });
    m_launchState = ok ? QStringLiteral("running") : QStringLiteral("failed");
    m_launchError = ok ? QString() : m_lastError;
    emit launchStateChanged();
    emit launchErrorChanged();
    if (!ok) {
        finish(QStringLiteral("playGame"), false, m_launchError);
    }
    return ok;
}

bool CoreBridge::repairGame(const QString &id)
{
    return forGame(id, QStringLiteral("repairGame"), [this](const QString &path) {
        if (installLibrary(path).isEmpty()) {
            return false;
        }
        applyLibrary(readLibrary());
        return true;
    });
}

bool CoreBridge::restoreGame(const QString &id)
{
    return forGame(id, QStringLiteral("restoreGame"), [this](const QString &path) {
        if (restoreLibrary(path).isEmpty()) {
            return m_lastError.isEmpty();
        }
        applyLibrary(readLibrary());
        return true;
    });
}

bool CoreBridge::removeGame(const QString &id)
{
    const bool ok = forGame(id, QStringLiteral("removeGame"), [this](const QString &path) {
        if (!recordStatus(sapie_core_library_remove(m_core, qUtf8Printable(path)),
                          QStringLiteral("Remove from library"))) {
            return false;
        }
        applyLibrary(readLibrary());
        return true;
    });
    if (ok && m_game.value(QStringLiteral("id")).toString() == id) {
        m_game.clear();
        m_gameState = QStringLiteral("empty");
        emit gameChanged();
        emit gameStateChanged();
    }
    return ok;
}

bool CoreBridge::saveGameOptions(const QString &id, const QString &args,
                                 const QString &preload)
{
    const QStringList pieces = splitArgs(args);
    std::vector<QByteArray> owned;
    owned.reserve(static_cast<size_t>(pieces.size()) + 1);
    for (const QString &piece : pieces) {
        owned.push_back(piece.toUtf8());
    }
    owned.push_back(preload.toUtf8());
    std::vector<const char *> pointers;
    for (const QByteArray &piece : owned) {
        pointers.push_back(piece.constData());
    }
    const char *preloadValue = preload.isEmpty() ? nullptr : owned.back().constData();
    return forGame(id, QStringLiteral("saveGameOptions"), [this, &pointers, preloadValue,
                                                           pieces](const QString &path) {
        const SapieStatus status = sapie_core_save_game_options(
                m_core, qUtf8Printable(path), pointers.data(), pieces.size(), preloadValue);
        if (!recordStatus(status, QStringLiteral("Save launch options"))) {
            return false;
        }
        applyLibrary(readLibrary());
        return true;
    });
}

void CoreBridge::chooseExecutable()
{
    emit pickExecutableRequested();
    finish(QStringLiteral("chooseExecutable"), true,
           QStringLiteral("Waiting for the platform file picker to supply a path."));
}

void CoreBridge::setChosenExecutable(const QString &path)
{
    m_detection = QVariantMap{{QStringLiteral("path"), path},
                              {QStringLiteral("arch"), QString()},
                              {QStringLiteral("era"), QString()},
                              {QStringLiteral("interfaces"), 0},
                              {QStringLiteral("appid"), 0},
                              {QStringLiteral("duplicate"), false}};
    m_addState = QStringLiteral("detecting");
    emit detectionChanged();
    emit addStateChanged();
    finish(QStringLiteral("setChosenExecutable"), true,
           QStringLiteral("Executable selected. Reading it now."));
}

QVariantMap CoreBridge::detectGame(const QString &path)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        finish(QStringLiteral("detectGame"), false, m_lastError);
        return {};
    }
    SapieDetectResult *rawResult = nullptr;
    const SapieStatus status = sapie_core_detect_game(m_core, qUtf8Printable(path), &rawResult);
    DetectOwner detected{rawResult};
    if (!recordStatus(status, QStringLiteral("Detect game"))) {
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        finish(QStringLiteral("detectGame"), false, m_lastError);
        return {};
    }

    const QString exePath = fromChar(detected.value->exe_path);
    bool duplicate = false;
    for (const QVariant &entry : readLibrary()) {
        if (entry.toMap().value(QStringLiteral("exePath")).toString() == exePath) {
            duplicate = true;
            break;
        }
    }
    m_detection = QVariantMap{
        {QStringLiteral("path"), exePath},
        {QStringLiteral("arch"), fromChar(detected.value->arch)},
        {QStringLiteral("era"), fromChar(detected.value->era)},
        {QStringLiteral("interfaces"), static_cast<int>(detected.value->interface_count)},
        {QStringLiteral("appid"), 0},
        {QStringLiteral("duplicate"), duplicate}
    };
    m_addState = QStringLiteral("review");
    emit detectionChanged();
    emit addStateChanged();
    finish(QStringLiteral("detectGame"), true, QStringLiteral("Detection finished."));

    QVariantMap result;
    result["exePath"] = exePath;
    result["arch"] = fromChar(detected.value->arch);
    result["era"] = fromChar(detected.value->era);
    result["interfaces"] = fromArray(detected.value->interfaces, detected.value->interface_count);
    result["originalLibrary"] = fromChar(detected.value->original_library);
    result["warning"] = fromChar(detected.value->warning);
    return result;
}

bool CoreBridge::installGame(const QVariantMap &review)
{
    const QString path = m_detection.value(QStringLiteral("path")).toString();
    if (path.isEmpty()) {
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        return finish(QStringLiteral("installGame"), false, QStringLiteral("Choose an executable first."));
    }
    for (const QVariant &entry : readLibrary()) {
        if (entry.toMap().value(QStringLiteral("exePath")).toString() == path) {
            m_addState = QStringLiteral("duplicate");
            emit addStateChanged();
            return finish(QStringLiteral("installGame"), false,
                          QStringLiteral("That executable is already in your library."));
        }
    }
    bool appidOk = false;
    const quint32 appid = toAppid(review.value(QStringLiteral("appid")), &appidOk);
    if (!appidOk || appid == 0) {
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        return finish(QStringLiteral("installGame"), false, QStringLiteral("Enter a valid App ID."));
    }

    m_addState = QStringLiteral("installing");
    emit addStateChanged();
    if (libraryAdd(path).isEmpty()) {
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        return finish(QStringLiteral("installGame"), false, m_lastError);
    }
    if (review.value(QStringLiteral("unpackStub")).toBool()
        && unpackSteamStub(path).isEmpty()) {
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        return finish(QStringLiteral("installGame"), false, m_lastError);
    }
    if (installLibrary(path).isEmpty() || configureGame(path, appid).isEmpty()) {
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        return finish(QStringLiteral("installGame"), false, m_lastError);
    }
    applyLibrary(readLibrary());
    if (!saveGameOptions(path, review.value(QStringLiteral("args")).toString(),
                         review.value(QStringLiteral("preload")).toString())) {
        m_addState = QStringLiteral("error");
        emit addStateChanged();
        return finish(QStringLiteral("installGame"), false, m_lastError);
    }

    m_pendingInstallPath = path;
    m_addState = QStringLiteral("done");
    emit addStateChanged();
    return finish(QStringLiteral("installGame"), true, QStringLiteral("Game added to the library."));
}

void CoreBridge::cancelInstall()
{
    m_addState = QStringLiteral("idle");
    emit addStateChanged();
    finish(QStringLiteral("cancelInstall"), true,
           QStringLiteral("No install was running. Choose another executable to continue."));
}

bool CoreBridge::rollbackInstall()
{
    if (m_pendingInstallPath.isEmpty()) {
        m_addState = QStringLiteral("idle");
        emit addStateChanged();
        return finish(QStringLiteral("rollbackInstall"), true,
                      QStringLiteral("There was no install to roll back."));
    }
    const bool ok = !restoreLibrary(m_pendingInstallPath).isEmpty() || m_lastError.isEmpty();
    if (ok) {
        m_pendingInstallPath.clear();
        applyLibrary(readLibrary());
    }
    m_addState = ok ? QStringLiteral("idle") : QStringLiteral("error");
    emit addStateChanged();
    return finish(QStringLiteral("rollbackInstall"), ok,
                  ok ? QStringLiteral("Original library restored.") : m_lastError);
}

QString CoreBridge::getPreference(const QString &key, const QString &fallback) const
{
    if (m_core != nullptr) {
        char *rawValue = nullptr;
        const SapieStatus status = sapie_core_get_preference(
                m_core, qUtf8Printable(key), qUtf8Printable(fallback), &rawValue);
        CharOwner value{rawValue};
        if (status == SAPIE_OK) {
            return fromChar(value.value);
        }
    }
    return m_prefs.value(key, fallback).toString();
}

bool CoreBridge::setPreference(const QString &key, const QString &value)
{
    if (m_core != nullptr) {
        const SapieStatus status = sapie_core_set_preference(
                m_core, qUtf8Printable(key), qUtf8Printable(value));
        if (!recordStatus(status, QStringLiteral("Save preference"))) {
            return finish(QStringLiteral("setPreference"), false, m_lastError);
        }
    }
    m_prefs.insert(key, value);
    emit prefsChanged();
    return finish(QStringLiteral("setPreference"), true, QStringLiteral("Preference saved."));
}

bool CoreBridge::setUpdateChannel(const QString &channel)
{
    if (channel != QStringLiteral("stable") && channel != QStringLiteral("beta")) {
        return finish(QStringLiteral("setUpdateChannel"), false,
                      QStringLiteral("Update channel must be stable or beta."));
    }
    m_updateChannel = channel;
    emit updateChannelChanged();
    setPreference(QStringLiteral("updateChannel"), channel);
    return finish(QStringLiteral("setUpdateChannel"), true,
                  QStringLiteral("Update channel set to %1.").arg(channel));
}

bool CoreBridge::checkForUpdates()
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        m_updateState = QStringLiteral("failed");
        emit updateStateChanged();
        return finish(QStringLiteral("checkForUpdates"), false, m_lastError);
    }
    char *rawVersion = nullptr;
    const SapieStatus status = sapie_core_check_updates(
            m_core, qUtf8Printable(m_updateChannel), &rawVersion);
    CharOwner version{rawVersion};
    if (!recordStatus(status, QStringLiteral("Check for updates"))) {
        m_updateState = QStringLiteral("failed");
        emit updateStateChanged();
        return finish(QStringLiteral("checkForUpdates"), false, m_lastError);
    }
    const QString available = fromChar(version.value);
    m_updateState = available.isEmpty() ? QStringLiteral("current") : QStringLiteral("available");
    emit updateStateChanged();
    return finish(QStringLiteral("checkForUpdates"), true,
                  available.isEmpty() ? QStringLiteral("No update is available.")
                                      : QStringLiteral("Update %1 is available.").arg(available));
}

bool CoreBridge::openDataFolder()
{
    if (m_dataFolder.isEmpty()) {
        return finish(QStringLiteral("openDataFolder"), false,
                      QStringLiteral("The data folder is not available."));
    }
    const bool opened = QDesktopServices::openUrl(QUrl::fromLocalFile(m_dataFolder));
    return finish(QStringLiteral("openDataFolder"), opened,
                  opened ? QStringLiteral("Opened %1.").arg(m_dataFolder)
                         : QStringLiteral("Could not open %1.").arg(m_dataFolder));
}

bool CoreBridge::copyLog()
{
    QStringList messages;
    for (const QVariant &entry : m_logs) {
        messages.append(entry.toMap().value(QStringLiteral("message")).toString());
    }
    m_copiedText = messages.join('\n');
    emit copiedTextChanged();
    if (QGuiApplication::instance() != nullptr && QGuiApplication::clipboard() != nullptr) {
        QGuiApplication::clipboard()->setText(m_copiedText);
    }
    return finish(QStringLiteral("copyLog"), true, QStringLiteral("Log copied."));
}

bool CoreBridge::copyDiagnostics()
{
    m_copiedText = QStringList{m_version, m_updateChannel, m_dataFolder}.join(QStringLiteral(" / "));
    emit copiedTextChanged();
    if (QGuiApplication::instance() != nullptr && QGuiApplication::clipboard() != nullptr) {
        QGuiApplication::clipboard()->setText(m_copiedText);
    }
    return finish(QStringLiteral("copyDiagnostics"), true, QStringLiteral("Diagnostics copied."));
}

bool CoreBridge::setReducedMotion(bool value)
{
    m_reducedMotion = value;
    emit reducedMotionChanged();
    setPreference(QStringLiteral("reducedMotion"), value ? QStringLiteral("true")
                                                          : QStringLiteral("false"));
    return finish(QStringLiteral("setReducedMotion"), true,
                  QStringLiteral("Reduced motion updated."));
}

bool CoreBridge::handleDeepLink(const QString &url)
{
    char *rawId = nullptr;
    const SapieStatus status = sapie_core_parse_deep_link(qUtf8Printable(url), &rawId);
    CharOwner id{rawId};
    if (!recordStatus(status, QStringLiteral("Read join link"))) {
        return finish(QStringLiteral("handleDeepLink"), false,
                      QStringLiteral("That join link is not valid."));
    }
    m_pendingLobbyId = fromChar(id.value);
    m_pendingInvite = url;
    emit pendingLobbyIdChanged();
    return finish(QStringLiteral("handleDeepLink"), true,
                  QStringLiteral("Join link accepted for lobby %1.").arg(m_pendingLobbyId));
}

QVariantMap CoreBridge::installLibrary(const QString &path)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return {};
    }
    SapieInstallResult *rawResult = nullptr;
    const SapieStatus status = sapie_core_install_library(m_core, qUtf8Printable(path), &rawResult);
    InstallOwner installed{rawResult};
    if (!recordStatus(status, QStringLiteral("Install library"))) {
        return {};
    }

    QVariantMap result;
    result["arch"] = fromChar(installed.value->arch);
    result["era"] = fromChar(installed.value->era);
    result["interfaces"] = fromArray(installed.value->interfaces, installed.value->interface_count);
    result["originalLibrary"] = fromChar(installed.value->original_library);
    result["library"] = fromChar(installed.value->library);
    result["warning"] = fromChar(installed.value->warning);
    return result;
}

QString CoreBridge::restoreLibrary(const QString &path)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return {};
    }
    char *rawRestored = nullptr;
    const SapieStatus status = sapie_core_restore_library(m_core, qUtf8Printable(path),
                                                          &rawRestored);
    CharOwner restored{rawRestored};
    if (!recordStatus(status, QStringLiteral("Restore library"))) {
        return {};
    }
    return fromChar(restored.value);
}

QVariantMap CoreBridge::configureGame(const QString &path, quint32 appid)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return {};
    }
    SapieConfigureResult *rawResult = nullptr;
    const SapieStatus status = sapie_core_configure_game(m_core, qUtf8Printable(path), appid,
                                                         &rawResult);
    ConfigureOwner configured{rawResult};
    if (!recordStatus(status, QStringLiteral("Configure game"))) {
        return {};
    }

    QVariantMap result;
    result["appid"] = configured.value->appid;
    result["gameServer"] = fromChar(configured.value->game_server);
    result["note"] = fromChar(configured.value->note);
    return result;
}

quint32 CoreBridge::launchGame(const QString &path)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return 0;
    }
    quint32 pid = 0;
    if (!recordStatus(sapie_core_launch_game(m_core, qUtf8Printable(path), &pid),
                      QStringLiteral("Launch game"))) {
        return 0;
    }
    return pid;
}

QString CoreBridge::unpackSteamStub(const QString &path)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return {};
    }
    char *rawUnpacked = nullptr;
    const SapieStatus status = sapie_core_unpack_steamstub(m_core, qUtf8Printable(path),
                                                           &rawUnpacked);
    CharOwner unpacked{rawUnpacked};
    if (!recordStatus(status, QStringLiteral("Unpack SteamStub"))) {
        return {};
    }
    return fromChar(unpacked.value);
}

QVariantMap CoreBridge::libraryAdd(const QString &path)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return {};
    }
    SapieGameRecord *rawRecord = nullptr;
    const SapieStatus status = sapie_core_library_add(m_core, qUtf8Printable(path), &rawRecord);
    RecordOwner record{rawRecord};
    if (!recordStatus(status, QStringLiteral("Add to library"))) {
        return {};
    }
    return recordToMap(*record.value);
}

bool CoreBridge::libraryRemove(const QString &path)
{
    if (m_core == nullptr) {
        setLastError(QStringLiteral("The core is not ready."));
        return false;
    }
    return recordStatus(sapie_core_library_remove(m_core, qUtf8Printable(path)),
                        QStringLiteral("Remove from library"));
}

bool CoreBridge::revokeToken()
{
    return finish(QStringLiteral("revokeToken"), false,
                  QStringLiteral("Server-backed token revocation is not enabled in this build."));
}

bool CoreBridge::listFriends()
{
    return finish(QStringLiteral("listFriends"), false,
                  QStringLiteral("Server-backed friends are not enabled in this build."));
}

bool CoreBridge::addFriend(const QString &)
{
    return finish(QStringLiteral("addFriend"), false,
                  QStringLiteral("Server-backed friends are not enabled in this build."));
}

bool CoreBridge::removeFriend(const QString &)
{
    return finish(QStringLiteral("removeFriend"), false,
                  QStringLiteral("Server-backed friends are not enabled in this build."));
}

bool CoreBridge::blockFriend(const QString &)
{
    return finish(QStringLiteral("blockFriend"), false,
                  QStringLiteral("Server-backed friends are not enabled in this build."));
}

bool CoreBridge::unblockFriend(const QString &)
{
    return finish(QStringLiteral("unblockFriend"), false,
                  QStringLiteral("Server-backed friends are not enabled in this build."));
}

bool CoreBridge::acceptRequest(const QString &)
{
    return finish(QStringLiteral("acceptRequest"), false,
                  QStringLiteral("Server-backed requests are not enabled in this build."));
}

bool CoreBridge::declineRequest(const QString &)
{
    return finish(QStringLiteral("declineRequest"), false,
                  QStringLiteral("Server-backed requests are not enabled in this build."));
}

bool CoreBridge::cancelRequest(const QString &)
{
    return finish(QStringLiteral("cancelRequest"), false,
                  QStringLiteral("Server-backed requests are not enabled in this build."));
}

bool CoreBridge::listLobbies()
{
    return finish(QStringLiteral("listLobbies"), false,
                  QStringLiteral("Server-backed lobbies are not enabled in this build."));
}

bool CoreBridge::joinLobby(const QString &)
{
    return finish(QStringLiteral("joinLobby"), false,
                  QStringLiteral("Server-backed lobbies are not enabled in this build."));
}

bool CoreBridge::inviteFriend(const QString &, const QString &)
{
    return finish(QStringLiteral("inviteFriend"), false,
                  QStringLiteral("Server-backed invites are not enabled in this build."));
}

bool CoreBridge::recordStatus(quint32 status, const QString &action)
{
    if (status == SAPIE_OK) {
        setLastError({});
        return true;
    }
    const QString message = QStringLiteral("%1: %2").arg(
            action, QString::fromUtf8(statusMessage(static_cast<SapieStatus>(status))));
    setLastError(message);
    log(QStringLiteral("Error"), message);
    emit errorOccurred(status, message);
    return false;
}

void CoreBridge::log(const QString &level, const QString &message)
{
    m_logs.append(QVariantMap{{QStringLiteral("level"), level},
                              {QStringLiteral("message"), message}});
    while (m_logs.size() > 200) {
        m_logs.removeFirst();
    }
    emit logsChanged();
}

void CoreBridge::refreshState()
{
    char *rawVersion = nullptr;
    if (sapie_core_launcher_version(&rawVersion) == SAPIE_OK) {
        CharOwner version{rawVersion};
        m_version = fromChar(version.value);
        emit versionChanged();
    }
    char *rawFolder = nullptr;
    if (sapie_core_data_folder(m_core, &rawFolder) == SAPIE_OK) {
        CharOwner folder{rawFolder};
        m_dataFolder = fromChar(folder.value);
        emit dataFolderChanged();
    }

    m_serverAddress = getPreference(QStringLiteral("server"), QString());
    m_accountName = getPreference(QStringLiteral("accountName"), QString());
    m_rememberSession = getPreference(QStringLiteral("rememberSession"), QStringLiteral("false"))
                                == QStringLiteral("true");
    m_firstRun = m_serverAddress.isEmpty();
    m_updateChannel = getPreference(QStringLiteral("updateChannel"), QStringLiteral("stable"));
    m_reducedMotion = getPreference(QStringLiteral("reducedMotion"), QStringLiteral("false"))
                              == QStringLiteral("true");

    for (const QString &key : {QStringLiteral("librarySearch"), QStringLiteral("libraryFilter"),
                               QStringLiteral("librarySort")}) {
        const QString fallback = key == QStringLiteral("libraryFilter")
                ? QStringLiteral("All games")
                : (key == QStringLiteral("librarySort") ? QStringLiteral("Name") : QString());
        m_prefs.insert(key, getPreference(key, fallback));
    }
    m_prefs.insert(QStringLiteral("server"), m_serverAddress);
    m_prefs.insert(QStringLiteral("accountName"), m_accountName);
    m_prefs.insert(QStringLiteral("updateChannel"), m_updateChannel);
    m_prefs.insert(QStringLiteral("reducedMotion"),
                   m_reducedMotion ? QStringLiteral("true") : QStringLiteral("false"));

    emit serverAddressChanged();
    emit accountNameChanged();
    emit rememberSessionChanged();
    emit firstRunChanged();
    emit updateChannelChanged();
    emit reducedMotionChanged();
    emit prefsChanged();
    m_settingsState = QStringLiteral("ready");
    m_logsState = QStringLiteral("ready");
    emit settingsStateChanged();
    emit logsStateChanged();
}

void CoreBridge::setLastError(const QString &message)
{
    if (m_lastError == message) {
        return;
    }
    m_lastError = message;
    emit errorChanged();
}

void CoreBridge::destroyCore()
{
    if (m_core == nullptr) {
        return;
    }
    sapie_core_destroy(m_core);
    m_core = nullptr;
}

void CoreBridge::handleEvent(void *userData, const char *step, const char *message)
{
    auto *bridge = static_cast<CoreBridge *>(userData);
    if (bridge != nullptr) {
        const QString stepText = fromChar(step);
        const QString messageText = fromChar(message);
        emit bridge->progress(stepText, messageText);
        bridge->log(QStringLiteral("Info"),
                    stepText.isEmpty() ? messageText
                                       : QStringLiteral("%1: %2").arg(stepText, messageText));
    }
}

bool CoreBridge::finish(const QString &operation, bool ok, const QString &message)
{
    m_lastAction = operation;
    emit lastActionChanged();
    emit operationFinished(operation, ok, message);
    return ok;
}

QString CoreBridge::joinArgs(const QVariant &args)
{
    const QStringList values = args.toStringList();
    return values.join(' ');
}

QStringList CoreBridge::splitArgs(const QString &args)
{
    return args.split(' ', Qt::SkipEmptyParts);
}

quint32 CoreBridge::toAppid(const QVariant &value, bool *ok)
{
    bool converted = false;
    const qulonglong number = value.toString().trimmed().toULongLong(&converted);
    if (!converted || number > std::numeric_limits<quint32>::max()) {
        *ok = false;
        return 0;
    }
    *ok = true;
    return static_cast<quint32>(number);
}

quint64 CoreBridge::toId(const QString &value, bool *ok)
{
    bool converted = false;
    const qulonglong number = value.trimmed().toULongLong(&converted);
    if (!converted || number == 0) {
        *ok = false;
        return 0;
    }
    *ok = true;
    return static_cast<quint64>(number);
}
