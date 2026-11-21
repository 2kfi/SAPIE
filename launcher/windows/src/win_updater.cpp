#include "win_updater.h"

#include "win_native.h"

#include <QCoreApplication>
#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QProcess>
#include <QRegularExpression>
#include <QTemporaryDir>

#include <windows.h>
#include <bcrypt.h>
#include <softpub.h>
#include <wincrypt.h>
#include <wintrust.h>

#include <vector>

namespace sapie::win {
namespace {

void fail(WindowsUpdater* updater, const QString& message)
{
    emit updater->failed(message);
}

bool newerVersion(const QString& candidate, const QString& current)
{
    const auto parts = [](const QString& value) {
        QVector<int> numbers;
        for (const QString& piece : value.split(QLatin1Char('.'))) {
            bool ok = false;
            numbers.append(piece.toInt(&ok));
            if (!ok) {
                numbers.append(0);
            }
        }
        while (numbers.size() < 3) {
            numbers.append(0);
        }
        return numbers;
    };

    const QVector<int> left = parts(candidate);
    const QVector<int> right = parts(current);
    for (int index = 0; index < 3; ++index) {
        if (left[index] != right[index]) {
            return left[index] > right[index];
        }
    }
    return false;
}

bool copyDirectory(const QString& source, const QString& destination, bool overwrite)
{
    QDir sourceDir(source);
    if (!sourceDir.exists()) {
        return false;
    }
    QDir().mkpath(destination);
    const QFileInfoList entries = sourceDir.entryInfoList(
        QDir::Dirs | QDir::Files | QDir::Hidden | QDir::NoDotAndDotDot, QDir::Name);
    for (const QFileInfo& entry : entries) {
        if (entry.isSymLink()) {
            continue;
        }
        const QString target = QDir(destination).filePath(entry.fileName());
        if (entry.isDir()) {
            if (!copyDirectory(entry.absoluteFilePath(), target, overwrite)) {
                return false;
            }
        } else {
            if (overwrite && QFile::exists(target) && !QFile::remove(target)) {
                return false;
            }
            if (!QFile::copy(entry.absoluteFilePath(), target)) {
                return false;
            }
        }
    }
    return true;
}

QString sanitisedVersion(const QString& version)
{
    QString value = version;
    value.remove(QRegularExpression(QStringLiteral("[^0-9.]")));
    return value.isEmpty() ? QStringLiteral("unknown") : value;
}

}

bool verifyManifestSignature(const QByteArray& body,
                             const QByteArray& signatureBase64,
                             const QString& publicKeyPemPath,
                             QString* error)
{
    const QByteArray signature = QByteArray::fromBase64(signatureBase64.trimmed());
    if (signature.isEmpty()) {
        if (error) {
            *error = QStringLiteral("The update manifest signature is not valid base64.");
        }
        return false;
    }

    QFile keyFile(publicKeyPemPath);
    if (!keyFile.open(QIODevice::ReadOnly)) {
        if (error) {
            *error = QStringLiteral("The update signing key is not readable.");
        }
        return false;
    }
    const QByteArray pem = keyFile.readAll();

    DWORD derSize = 0;
    if (!CryptStringToBinaryA(pem.constData(), 0, CRYPT_STRING_BASE64HEADER,
                              nullptr, &derSize, nullptr, nullptr)
        || derSize == 0) {
        if (error) {
            *error = QStringLiteral("The update signing key is not a PEM public key.");
        }
        return false;
    }
    std::vector<BYTE> der(derSize);
    if (!CryptStringToBinaryA(pem.constData(), 0, CRYPT_STRING_BASE64HEADER,
                              der.data(), &derSize, nullptr, nullptr)) {
        if (error) {
            *error = QStringLiteral("The update signing key could not be decoded.");
        }
        return false;
    }

    void* decoded = nullptr;
    DWORD infoSize = 0;
    if (!CryptDecodeObjectEx(X509_ASN_ENCODING, X509_PUBLIC_KEY_INFO, der.data(), derSize,
                             CRYPT_DECODE_ALLOC_FLAG, nullptr, &decoded, &infoSize)
        || !decoded) {
        if (error) {
            *error = QStringLiteral("The update signing key has an unknown format.");
        }
        return false;
    }
    auto* publicKeyInfo = static_cast<CERT_PUBLIC_KEY_INFO*>(decoded);

    DWORD blobSize = 0;
    const bool sized = CryptDecodeObjectEx(X509_ASN_ENCODING, RSA_CSP_PUBLICKEYBLOB,
                                           publicKeyInfo->PublicKey.pbData,
                                           publicKeyInfo->PublicKey.cbData,
                                           0, nullptr, nullptr, &blobSize);
    std::vector<BYTE> blob(blobSize ? blobSize : 1);
    const bool decodedBlob = sized && blobSize != 0
        && CryptDecodeObjectEx(X509_ASN_ENCODING, RSA_CSP_PUBLICKEYBLOB,
                               publicKeyInfo->PublicKey.pbData,
                               publicKeyInfo->PublicKey.cbData,
                               0, nullptr, blob.data(), &blobSize);
    LocalFree(decoded);
    if (!decodedBlob) {
        if (error) {
            *error = QStringLiteral("The update signing key is not an RSA public key.");
        }
        return false;
    }

    BCRYPT_ALG_HANDLE algorithm = nullptr;
    BCRYPT_KEY_HANDLE key = nullptr;
    const bool opened = BCRYPT_SUCCESS(
        BCryptOpenAlgorithmProvider(&algorithm, BCRYPT_RSA_ALGORITHM, nullptr, 0));
    const bool imported = opened && BCRYPT_SUCCESS(
        BCryptImportKeyPair(algorithm, nullptr, LEGACY_RSAPUBLIC_BLOB, &key,
                            blob.data(), static_cast<ULONG>(blobSize), 0));
    if (!imported) {
        if (algorithm) {
            BCryptCloseAlgorithmProvider(algorithm, 0);
        }
        if (error) {
            *error = QStringLiteral("The update signing key could not be imported.");
        }
        return false;
    }

    const QByteArray digest = QCryptographicHash::hash(body, QCryptographicHash::Sha256);
    BCRYPT_PKCS1_PADDING_INFO padding{};
    padding.pszAlgId = BCRYPT_SHA256_ALGORITHM;
    const NTSTATUS status = BCryptVerifySignature(
        key, &padding, reinterpret_cast<PUCHAR>(digest.data()), static_cast<ULONG>(digest.size()),
        reinterpret_cast<PUCHAR>(signature.data()), static_cast<ULONG>(signature.size()), 0);

    BCryptDestroyKey(key);
    BCryptCloseAlgorithmProvider(algorithm, 0);
    if (!BCRYPT_SUCCESS(status)) {
        if (error) {
            *error = QStringLiteral("The update manifest signature did not verify. Status %1.")
                .arg(status);
        }
        return false;
    }
    return true;
}

WindowsUpdater::WindowsUpdater(const QString& channel,
                               const QString& arch,
                               const QString& publicKeyPemPath,
                               QObject* parent)
    : QObject(parent),
      m_network(new QNetworkAccessManager(this)),
      m_channel(channel),
      m_arch(arch),
      m_publicKeyPemPath(publicKeyPemPath)
{
}

void WindowsUpdater::check(const QUrl& manifestUrl, const QString& currentVersion)
{
    if (m_channel != QStringLiteral("stable") && m_channel != QStringLiteral("beta")) {
        fail(this, QStringLiteral("The update channel must be stable or beta."));
        return;
    }
    if (!manifestUrl.isValid()
        || (manifestUrl.scheme() != QStringLiteral("https") && !manifestUrl.isLocalFile())) {
        fail(this, QStringLiteral("The update manifest URL must use HTTPS."));
        return;
    }

    m_manifestUrl = manifestUrl;
    m_currentVersion = currentVersion;
    emit progress(QStringLiteral("Checking for updates"), 0);
    QNetworkReply* reply = m_network->get(QNetworkRequest(manifestUrl));
    connect(reply, &QNetworkReply::finished, this, &WindowsUpdater::onManifestReceived);
}

void WindowsUpdater::apply(const UpdateManifest& manifest, const QString& currentVersion)
{
    if (manifest.channel != m_channel) {
        fail(this, QStringLiteral("The update belongs to a different channel."));
        return;
    }
    if (!selectArtifact(manifest, &m_pendingArtifact, nullptr)) {
        fail(this, QStringLiteral("There is no Windows update for this architecture."));
        return;
    }
    m_currentVersion = currentVersion;
    m_pendingVersion = manifest.version;
    QString error;
    if (!prepareRollback(currentVersion, &error)) {
        fail(this, error);
        return;
    }

    emit progress(QStringLiteral("Downloading update"), 0);
    QNetworkReply* reply = m_network->get(QNetworkRequest(m_pendingArtifact.url));
    connect(reply, &QNetworkReply::finished, this, &WindowsUpdater::onArtifactDownloaded);
}

void WindowsUpdater::onManifestReceived()
{
    auto* reply = qobject_cast<QNetworkReply*>(sender());
    if (!reply) {
        return;
    }
    const QByteArray body = reply->readAll();
    reply->deleteLater();
    if (reply->error() != QNetworkReply::NoError) {
        fail(this, QStringLiteral("The update manifest could not be downloaded."));
        return;
    }

    m_manifestBody = body;
    QUrl signatureUrl = m_manifestUrl;
    signatureUrl.setPath(signatureUrl.path() + QStringLiteral(".sig"));
    QNetworkReply* signatureReply = m_network->get(QNetworkRequest(signatureUrl));
    connect(signatureReply, &QNetworkReply::finished, this, &WindowsUpdater::onSignatureReceived);
}

void WindowsUpdater::onSignatureReceived()
{
    auto* reply = qobject_cast<QNetworkReply*>(sender());
    if (!reply) {
        return;
    }
    const QByteArray signature = reply->readAll();
    reply->deleteLater();
    if (reply->error() != QNetworkReply::NoError) {
        fail(this, QStringLiteral("The update manifest signature could not be downloaded."));
        return;
    }

    QString error;
    if (!verifyManifestSignature(m_manifestBody, signature, m_publicKeyPemPath, &error)) {
        fail(this, error);
        return;
    }
    const UpdateManifest manifest = parseManifest(m_manifestBody, &error);
    if (!error.isEmpty()) {
        fail(this, error);
        return;
    }
    if (manifest.channel != m_channel) {
        fail(this, QStringLiteral("The update manifest channel does not match this install."));
        return;
    }
    if (newerVersion(manifest.version, m_currentVersion)) {
        emit updateAvailable(manifest);
    } else {
        emit upToDate();
    }
}

void WindowsUpdater::onArtifactDownloaded()
{
    auto* reply = qobject_cast<QNetworkReply*>(sender());
    if (!reply) {
        return;
    }
    const QByteArray bytes = reply->readAll();
    reply->deleteLater();
    if (reply->error() != QNetworkReply::NoError) {
        fail(this, QStringLiteral("The update could not be downloaded."));
        return;
    }

    QString error;
    if (!validateArtifact(bytes, m_pendingArtifact, &error)) {
        fail(this, error);
        return;
    }

    const QString updateDir = QDir(dataDirectory()).filePath(QStringLiteral("updates"));
    QDir().mkpath(updateDir);
    const QString finalPath = QDir(updateDir).filePath(m_pendingArtifact.name);
    const QString partPath = finalPath + QStringLiteral(".part");
    QFile part(partPath);
    if (!part.open(QIODevice::WriteOnly | QIODevice::Truncate)
        || part.write(bytes) != bytes.size() || !part.flush()) {
        part.remove();
        fail(this, QStringLiteral("The update could not be written to disk."));
        return;
    }
    part.close();
    QFile::remove(finalPath);
    if (!QFile::rename(partPath, finalPath) || !verifyAuthenticode(finalPath, &error)) {
        QFile::remove(finalPath);
        fail(this, error.isEmpty() ? QStringLiteral("The update failed signature verification.") : error);
        return;
    }
    m_pendingPath = finalPath;
    emit progress(QStringLiteral("Installing update"), 90);

    bool installed = false;
    if (m_pendingArtifact.name.endsWith(QStringLiteral("-setup.exe"), Qt::CaseInsensitive)) {
        QProcess installer;
        installer.setProgram(finalPath);
        installer.setArguments({
            QStringLiteral("/S"),
            QStringLiteral("/D=") + QDir::toNativeSeparators(dataDirectory())
        });
        installer.start();
        installed = installer.waitForStarted(5000)
            && installer.waitForFinished(300000)
            && installer.exitCode() == 0;
    } else {
        QTemporaryDir extracted(QDir(dataDirectory()).filePath(QStringLiteral("update-XXXXXX")));
        extracted.setAutoRemove(false);
        QProcess tar;
        tar.setProgram(QStringLiteral("tar.exe"));
        tar.setArguments({QStringLiteral("-xf"), finalPath, QStringLiteral("-C"), extracted.path()});
        tar.start();
        const bool unpacked = tar.waitForStarted(5000) && tar.waitForFinished(300000)
            && tar.exitCode() == 0;
        const QString newExe = QDir(extracted.path()).filePath(QStringLiteral("SAPIE.exe"));
        if (!unpacked || !verifyAuthenticode(newExe, &error)) {
            QDir(extracted.path()).removeRecursively();
            fail(this, error.isEmpty() ? QStringLiteral("The portable update could not be unpacked.") : error);
            return;
        }

        const QString currentDir = QCoreApplication::applicationDirPath();
        const QString previousDir = currentDir + QStringLiteral(".sapie-old");
        QDir(previousDir).removeRecursively();
        const bool movedOld = QDir().rename(currentDir, previousDir);
        const bool movedNew = movedOld && QDir().rename(extracted.path(), currentDir);
        if (!movedNew) {
            if (movedOld) {
                QDir().rename(previousDir, currentDir);
            }
            fail(this, QStringLiteral("The portable update could not replace the current folder."));
            return;
        }
        QDir(previousDir).removeRecursively();
        installed = true;
    }

    if (!installed) {
        QString rollbackError;
        restoreRollback(&rollbackError);
        fail(this, QStringLiteral("The update failed. %1").arg(rollbackError));
        return;
    }
    emit progress(QStringLiteral("Update installed"), 100);
    emit applied(m_pendingVersion);
}

UpdateManifest WindowsUpdater::parseManifest(const QByteArray& body, QString* error) const
{
    UpdateManifest manifest;
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(body, &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        if (error) {
            *error = QStringLiteral("The update manifest is not valid JSON.");
        }
        return manifest;
    }

    const QJsonObject object = document.object();
    manifest.schema = object.value(QStringLiteral("schema")).toString();
    manifest.channel = object.value(QStringLiteral("channel")).toString();
    manifest.version = object.value(QStringLiteral("version")).toString();
    manifest.tag = object.value(QStringLiteral("tag")).toString();
    manifest.previousVersion = object.value(QStringLiteral("previous_version")).toString();
    manifest.rollback = object.value(QStringLiteral("rollback")).toString();
    manifest.body = body;

    if (manifest.schema != QStringLiteral("sapie-update/v1")) {
        if (error) {
            *error = QStringLiteral("The update manifest schema is not supported.");
        }
        return manifest;
    }
    if (manifest.channel != QStringLiteral("stable") && manifest.channel != QStringLiteral("beta")) {
        if (error) {
            *error = QStringLiteral("The update manifest channel is not supported.");
        }
        return manifest;
    }

    static const QRegularExpression shaPattern(QStringLiteral("^[0-9a-f]{64}$"));
    const QJsonArray artifacts = object.value(QStringLiteral("artifacts")).array();
    for (const QJsonValue& value : artifacts) {
        const QJsonObject item = value.toObject();
        UpdateArtifact artifact;
        artifact.name = item.value(QStringLiteral("name")).toString();
        artifact.os = item.value(QStringLiteral("os")).toString();
        artifact.arch = item.value(QStringLiteral("arch")).toString();
        artifact.sha256 = item.value(QStringLiteral("sha256")).toString();
        artifact.size = static_cast<qlonglong>(item.value(QStringLiteral("size")).toDouble());
        artifact.url = QUrl(item.value(QStringLiteral("url")).toString());
        if (artifact.name.isEmpty() || !shaPattern.match(artifact.sha256).hasMatch()
            || !artifact.url.isValid()) {
            if (error) {
                *error = QStringLiteral("The update manifest has an invalid artifact.");
            }
            return manifest;
        }
        manifest.artifacts.append(artifact);
    }
    if (manifest.artifacts.isEmpty()) {
        if (error) {
            *error = QStringLiteral("The update manifest has no artifacts.");
        }
    }
    return manifest;
}

bool WindowsUpdater::selectArtifact(const UpdateManifest& manifest,
                                    UpdateArtifact* artifact,
                                    QString* error) const
{
    for (const UpdateArtifact& candidate : manifest.artifacts) {
        const bool setup = candidate.name.endsWith(QStringLiteral("-setup.exe"), Qt::CaseInsensitive);
        const bool portable = candidate.name.endsWith(QStringLiteral("-portable.zip"), Qt::CaseInsensitive);
        if (candidate.os == QStringLiteral("windows") && candidate.arch == m_arch
            && (setup || portable)) {
            *artifact = candidate;
            return true;
        }
    }
    if (error) {
        *error = QStringLiteral("No Windows installer for this architecture was published.");
    }
    return false;
}

bool WindowsUpdater::validateArtifact(const QByteArray& bytes,
                                      const UpdateArtifact& artifact,
                                      QString* error) const
{
    if (artifact.size > 0 && bytes.size() != artifact.size) {
        if (error) {
            *error = QStringLiteral("The downloaded update has the wrong size.");
        }
        return false;
    }
    const QString actual = QCryptographicHash::hash(bytes, QCryptographicHash::Sha256).toHex();
    if (actual.compare(artifact.sha256, Qt::CaseInsensitive) != 0) {
        if (error) {
            *error = QStringLiteral("The downloaded update failed its SHA256 check.");
        }
        return false;
    }
    return true;
}

bool WindowsUpdater::verifyAuthenticode(const QString& path, QString* error) const
{
    const std::wstring native = path.toStdWString();
    WINTRUST_FILE_INFO fileInfo{};
    fileInfo.cbStruct = sizeof(fileInfo);
    fileInfo.pcwszFilePath = native.c_str();

    GUID action = WINTRUST_ACTION_GENERIC_VERIFY_V2;
    WINTRUST_DATA data{};
    data.cbStruct = sizeof(data);
    data.dwUIChoice = WTD_UI_NONE;
    data.fdwRevocationChecks = WTD_REVOKE_NONE;
    data.dwUnionChoice = WTD_CHOICE_FILE;
    data.pFile = &fileInfo;
    data.dwStateAction = WTD_STATEACTION_VERIFY;
    LONG status = WinVerifyTrust(nullptr, &action, &data);
    data.dwStateAction = WTD_STATEACTION_CLOSE;
    WinVerifyTrust(nullptr, &action, &data);

    if (status != ERROR_SUCCESS) {
        if (error) {
            *error = QStringLiteral("Authenticode verification failed with status %1.").arg(status);
        }
        return false;
    }
    return true;
}

bool WindowsUpdater::prepareRollback(const QString& currentVersion, QString* error)
{
    const QString root = QDir(dataDirectory()).filePath(QStringLiteral("rollback"));
    QDir().mkpath(root);
    m_rollbackDir = QDir(root).filePath(sanitisedVersion(currentVersion));
    if (QDir(m_rollbackDir).exists() && !QDir(m_rollbackDir).removeRecursively()) {
        if (error) {
            *error = QStringLiteral("The previous rollback copy could not be removed.");
        }
        return false;
    }
    if (!copyDirectory(QCoreApplication::applicationDirPath(), m_rollbackDir, false)) {
        if (error) {
            *error = QStringLiteral("The current version could not be saved for rollback.");
        }
        return false;
    }
    return true;
}

bool WindowsUpdater::restoreRollback(QString* error)
{
    if (m_rollbackDir.isEmpty() || !QDir(m_rollbackDir).exists()) {
        if (error) {
            *error = QStringLiteral("No signed previous version is available. See the rollback notes.");
        }
        return false;
    }
    if (!copyDirectory(m_rollbackDir, QCoreApplication::applicationDirPath(), true)) {
        if (error) {
            *error = QStringLiteral("The previous version could not be restored. See the rollback notes.");
        }
        return false;
    }
    return true;
}

}
