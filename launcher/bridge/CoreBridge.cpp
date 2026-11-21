#include "CoreBridge.h"

extern "C" {
#include "sapie_core.h"
}

#include <QStringList>

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
    if (!recordStatus(status, "Create core")) {
        return false;
    }
    if (m_core == nullptr) {
        setLastError("Create core: the core returned no handle.");
        emit errorOccurred(SAPIE_ERR_IO, m_lastError);
        return false;
    }
    return true;
}

bool CoreBridge::testConnection(const QString &server)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return false;
    }
    return recordStatus(sapie_core_test_connection(m_core, qUtf8Printable(server)),
                        "Test connection");
}

QVariantMap CoreBridge::login(const QString &server, const QString &username,
                              const QString &password)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return {};
    }
    SapieSessionInfo *rawSession = nullptr;
    const SapieStatus status = sapie_core_login(m_core, qUtf8Printable(server),
                                                qUtf8Printable(username),
                                                qUtf8Printable(password), &rawSession);
    SessionOwner session{rawSession};
    if (!recordStatus(status, "Sign in")) {
        return {};
    }

    QVariantMap result;
    result["steamId"] = static_cast<qulonglong>(session.value->steam_id);
    result["personaName"] = fromChar(session.value->persona_name);
    result["server"] = fromChar(session.value->server);
    result["expiresAt"] = static_cast<qulonglong>(session.value->expires_at);
    return result;
}

bool CoreBridge::signOut()
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return false;
    }
    return recordStatus(sapie_core_sign_out(m_core), "Sign out");
}

QVariantMap CoreBridge::detectGame(const QString &path)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return {};
    }
    SapieDetectResult *rawResult = nullptr;
    const SapieStatus status = sapie_core_detect_game(m_core, qUtf8Printable(path), &rawResult);
    DetectOwner detected{rawResult};
    if (!recordStatus(status, "Detect game")) {
        return {};
    }

    QVariantMap result;
    result["exePath"] = fromChar(detected.value->exe_path);
    result["arch"] = fromChar(detected.value->arch);
    result["era"] = fromChar(detected.value->era);
    result["interfaces"] = fromArray(detected.value->interfaces, detected.value->interface_count);
    result["originalLibrary"] = fromChar(detected.value->original_library);
    result["warning"] = fromChar(detected.value->warning);
    return result;
}

QVariantMap CoreBridge::installLibrary(const QString &path)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return {};
    }
    SapieInstallResult *rawResult = nullptr;
    const SapieStatus status = sapie_core_install_library(m_core, qUtf8Printable(path), &rawResult);
    InstallOwner installed{rawResult};
    if (!recordStatus(status, "Install library")) {
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
        setLastError("The core is not ready.");
        return {};
    }
    char *rawRestored = nullptr;
    const SapieStatus status = sapie_core_restore_library(m_core, qUtf8Printable(path),
                                                          &rawRestored);
    CharOwner restored{rawRestored};
    if (!recordStatus(status, "Restore library")) {
        return {};
    }
    return fromChar(restored.value);
}

QVariantMap CoreBridge::configureGame(const QString &path, quint32 appid)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return {};
    }
    SapieConfigureResult *rawResult = nullptr;
    const SapieStatus status = sapie_core_configure_game(m_core, qUtf8Printable(path), appid,
                                                         &rawResult);
    ConfigureOwner configured{rawResult};
    if (!recordStatus(status, "Configure game")) {
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
        setLastError("The core is not ready.");
        return 0;
    }
    quint32 pid = 0;
    if (!recordStatus(sapie_core_launch_game(m_core, qUtf8Printable(path), &pid), "Launch game")) {
        return 0;
    }
    return pid;
}

QString CoreBridge::unpackSteamStub(const QString &path)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return {};
    }
    char *rawUnpacked = nullptr;
    const SapieStatus status = sapie_core_unpack_steamstub(m_core, qUtf8Printable(path),
                                                           &rawUnpacked);
    CharOwner unpacked{rawUnpacked};
    if (!recordStatus(status, "Unpack SteamStub")) {
        return {};
    }
    return fromChar(unpacked.value);
}

QVariantList CoreBridge::libraryList()
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return {};
    }
    SapieGameRecord *rawRecords = nullptr;
    size_t count = 0;
    const SapieStatus status = sapie_core_library_list(m_core, &rawRecords, &count);
    RecordArrayOwner records{rawRecords, count};
    if (!recordStatus(status, "List library")) {
        return {};
    }

    QVariantList result;
    for (size_t i = 0; i < count; ++i) {
        result.append(recordToMap(records.value[i]));
    }
    return result;
}

QVariantMap CoreBridge::libraryAdd(const QString &path)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return {};
    }
    SapieGameRecord *rawRecord = nullptr;
    const SapieStatus status = sapie_core_library_add(m_core, qUtf8Printable(path), &rawRecord);
    RecordOwner record{rawRecord};
    if (!recordStatus(status, "Add to library")) {
        return {};
    }
    return recordToMap(*record.value);
}

bool CoreBridge::libraryRemove(const QString &path)
{
    if (m_core == nullptr) {
        setLastError("The core is not ready.");
        return false;
    }
    return recordStatus(sapie_core_library_remove(m_core, qUtf8Printable(path)),
                        "Remove from library");
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
    emit errorOccurred(status, message);
    return false;
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
        emit bridge->progress(fromChar(step), fromChar(message));
    }
}
