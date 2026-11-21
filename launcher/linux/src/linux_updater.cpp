#include "linux_updater.h"

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
#include <QTemporaryFile>

namespace sapie::lnx {
namespace {

constexpr qint64 kMaxArtifactBytes = 1024LL * 1024LL * 1024LL;

QString reject(QString* error, const QString& message)
{
    if (error) {
        *error = message;
    }
    return {};
}

bool isHexHash(const QString& value)
{
    static const QRegularExpression hex(QStringLiteral("^[0-9a-f]{64}$"));
    return hex.match(value).hasMatch();
}

}

bool parseUpdateManifest(const QByteArray& body, const QString& channel,
                         UpdateManifest* out, QString* error)
{
    if (!out) {
        return reject(error, QStringLiteral("No manifest was supplied.")), false;
    }
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(body, &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        return reject(error, QStringLiteral("The update manifest is not valid JSON.")), false;
    }
    const QJsonObject object = document.object();
    if (object.value(QStringLiteral("schema")).toString() != QStringLiteral("sapie-update/v1")) {
        return reject(error, QStringLiteral("The update manifest schema is unsupported.")), false;
    }
    if (object.value(QStringLiteral("channel")).toString() != channel) {
        return reject(error, QStringLiteral("The update manifest channel does not match.")), false;
    }

    UpdateManifest manifest;
    manifest.schema = object.value(QStringLiteral("schema")).toString();
    manifest.channel = object.value(QStringLiteral("channel")).toString();
    manifest.version = object.value(QStringLiteral("version")).toString();
    manifest.previousVersion = object.value(QStringLiteral("previous_version")).toString();
    manifest.rollback = object.value(QStringLiteral("rollback")).toString();
    manifest.body = body;
    if (manifest.version.isEmpty()) {
        return reject(error, QStringLiteral("The update manifest has no version.")), false;
    }

    static const QRegularExpression namePattern(
        QStringLiteral("^SAPIE-.+-linux-(x64|arm64)\\.(AppImage|flatpak)$"));
    const QJsonArray artifacts = object.value(QStringLiteral("artifacts")).toArray();
    if (artifacts.isEmpty()) {
        return reject(error, QStringLiteral("The update manifest has no artifacts.")), false;
    }
    for (const QJsonValue& value : artifacts) {
        const QJsonObject entry = value.toObject();
        UpdateArtifact artifact;
        artifact.name = entry.value(QStringLiteral("name")).toString();
        artifact.os = entry.value(QStringLiteral("os")).toString();
        artifact.arch = entry.value(QStringLiteral("arch")).toString();
        artifact.sha256 = entry.value(QStringLiteral("sha256")).toString();
        artifact.size = static_cast<qint64>(entry.value(QStringLiteral("size")).toDouble());
        artifact.url = QUrl(entry.value(QStringLiteral("url")).toString());
        if (!namePattern.match(artifact.name).hasMatch()
            || artifact.os != QStringLiteral("linux") || !isHexHash(artifact.sha256)
            || artifact.size <= 0 || artifact.size > kMaxArtifactBytes
            || !artifact.url.isValid() || artifact.url.scheme() != QStringLiteral("https")
            || artifact.url.host().isEmpty()) {
            return reject(error, QStringLiteral("The update manifest has an invalid artifact.")), false;
        }
        manifest.artifacts.append(artifact);
    }
    *out = manifest;
    return true;
}

bool verifyManifestSignature(const QByteArray& body, const QByteArray& signatureBase64,
                             const QString& publicKeyPemPath, QString* error)
{
    if (publicKeyPemPath.isEmpty() || !QFileInfo::exists(publicKeyPemPath)) {
        return reject(error, QStringLiteral("The update signing key is not available.")), false;
    }
    const QByteArray rawSignature = QByteArray::fromBase64(signatureBase64, QByteArray::AbortOnBase64DecodingErrors);
    if (rawSignature.isEmpty()) {
        return reject(error, QStringLiteral("The update signature is not valid base64.")), false;
    }

    QTemporaryFile manifestFile;
    QTemporaryFile signatureFile;
    if (!manifestFile.open() || !signatureFile.open()) {
        return reject(error, QStringLiteral("A temporary file could not be opened.")), false;
    }
    manifestFile.write(body);
    signatureFile.write(rawSignature);
    manifestFile.flush();
    signatureFile.flush();

    QProcess openssl;
    openssl.start(QStringLiteral("openssl"),
                  {QStringLiteral("pkeyutl"), QStringLiteral("-verify"), QStringLiteral("-pubin"),
                   QStringLiteral("-rawin"), QStringLiteral("-inkey"), publicKeyPemPath,
                   QStringLiteral("-in"), manifestFile.fileName(),
                   QStringLiteral("-sigfile"), signatureFile.fileName()});
    if (!openssl.waitForFinished(10000) || openssl.exitCode() != 0) {
        return reject(error, QStringLiteral("The update signature did not verify.")), false;
    }
    return true;
}

LinuxUpdater::LinuxUpdater(const QString& channel, const QString& arch, const QString& kind,
                           const QString& publicKeyPemPath, QObject* parent)
    : QObject(parent)
    , m_network(new QNetworkAccessManager(this))
    , m_channel(channel)
    , m_arch(arch)
    , m_kind(kind)
    , m_publicKeyPemPath(publicKeyPemPath)
{
}

LinuxUpdater::~LinuxUpdater() = default;

void LinuxUpdater::check(const QUrl& manifestUrl, const QString& currentVersion)
{
    m_currentVersion = currentVersion;
    if (!manifestUrl.isValid() || manifestUrl.scheme() != QStringLiteral("https")) {
        emit failed(QStringLiteral("The update manifest URL must use HTTPS."));
        return;
    }

    QNetworkRequest request(manifestUrl);
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute,
                         QNetworkRequest::NoLessSafeRedirectPolicy);
    QNetworkReply* reply = m_network->get(request);
    connect(reply, &QNetworkReply::finished, this, [this, reply, manifestUrl]() {
        reply->deleteLater();
        if (reply->error() != QNetworkReply::NoError) {
            emit failed(QStringLiteral("The update manifest could not be downloaded."));
            return;
        }
        m_manifestBody = reply->readAll();
        emit progress(QStringLiteral("manifest"), 30);

        QUrl signatureUrl = manifestUrl;
        signatureUrl.setPath(manifestUrl.path() + QStringLiteral(".sig"));
        QNetworkRequest signatureRequest(signatureUrl);
        signatureRequest.setAttribute(QNetworkRequest::RedirectPolicyAttribute,
                                      QNetworkRequest::NoLessSafeRedirectPolicy);
        QNetworkReply* signatureReply = m_network->get(signatureRequest);
        connect(signatureReply, &QNetworkReply::finished, this,
                [this, signatureReply]() { onSignatureReceived(); });
    });
}

void LinuxUpdater::onSignatureReceived()
{
    auto* reply = qobject_cast<QNetworkReply*>(sender());
    if (!reply) {
        emit failed(QStringLiteral("The update signature response was lost."));
        return;
    }
    reply->deleteLater();
    if (reply->error() != QNetworkReply::NoError) {
        emit failed(QStringLiteral("The update signature could not be downloaded."));
        return;
    }
    QString error;
    if (!verifyManifestSignature(m_manifestBody, reply->readAll(), m_publicKeyPemPath, &error)) {
        emit failed(error);
        return;
    }
    emit progress(QStringLiteral("signature"), 60);

    UpdateManifest manifest;
    if (!parseUpdateManifest(m_manifestBody, m_channel, &manifest, &error)) {
        emit failed(error);
        return;
    }
    if (manifest.version == m_currentVersion) {
        emit upToDate();
        return;
    }
    emit updateAvailable(manifest);
}

bool LinuxUpdater::selectArtifact(const UpdateManifest& manifest, UpdateArtifact* artifact,
                                  QString* error) const
{
    const QString suffix = QStringLiteral(".%1").arg(m_kind);
    for (const UpdateArtifact& candidate : manifest.artifacts) {
        if (candidate.arch == m_arch && candidate.name.endsWith(suffix)) {
            *artifact = candidate;
            return true;
        }
    }
    return reject(error, QStringLiteral("The update has no %1 artifact for %2.")
                             .arg(m_kind, m_arch)), false;
}

void LinuxUpdater::apply(const UpdateManifest& manifest, const QString& currentVersion)
{
    m_currentVersion = currentVersion;
    UpdateArtifact artifact;
    QString error;
    if (!selectArtifact(manifest, &artifact, &error)) {
        emit failed(error);
        return;
    }
    if (manifest.version == currentVersion) {
        emit upToDate();
        return;
    }
    m_pendingVersion = manifest.version;
    if (m_kind == QStringLiteral("Flatpak")) {
        updateFlatpak();
        return;
    }
    if (m_kind != QStringLiteral("AppImage")) {
        emit failed(QStringLiteral("This install type updates through its package manager."));
        return;
    }
    if (!prepareRollback(&error)) {
        emit failed(error);
        return;
    }
    m_pendingArtifact = artifact;
    emit progress(QStringLiteral("download"), 0);

    QNetworkRequest request(artifact.url);
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute,
                         QNetworkRequest::NoLessSafeRedirectPolicy);
    QNetworkReply* reply = m_network->get(request);
    connect(reply, &QNetworkReply::downloadProgress, this,
            [this](qint64 received, qint64 total) {
                const int percent = total > 0
                    ? static_cast<int>(received * 90 / total)
                    : 0;
                emit progress(QStringLiteral("download"), qMin(percent, 90));
            });
    connect(reply, &QNetworkReply::finished, this, [this, reply]() {
        reply->deleteLater();
        if (reply->error() != QNetworkReply::NoError) {
            restoreRollback();
            emit failed(QStringLiteral("The update artifact could not be downloaded."));
            return;
        }
        const QByteArray bytes = reply->readAll();
        if (bytes.size() != m_pendingArtifact.size
            || QCryptographicHash::hash(bytes, QCryptographicHash::Sha256).toHex()
                   != m_pendingArtifact.sha256.toUtf8()) {
            restoreRollback();
            emit failed(QStringLiteral("The update artifact failed its checksum."));
            return;
        }

        const QString current = QCoreApplication::applicationFilePath();
        const QString next = current + QStringLiteral(".next");
        QFile file(next);
        if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate)
            || file.write(bytes) != bytes.size() || !file.flush()) {
            restoreRollback();
            emit failed(QStringLiteral("The update artifact could not be staged."));
            return;
        }
        file.close();
        file.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner
                            | QFileDevice::ReadGroup | QFileDevice::ExeGroup
                            | QFileDevice::ReadOther | QFileDevice::ExeOther);
        if (!QFile::rename(next, current)) {
            QFile::remove(next);
            restoreRollback();
            emit failed(QStringLiteral("The update artifact could not be installed."));
            return;
        }
        m_rollbackPath.clear();
        emit progress(QStringLiteral("applied"), 100);
        emit applied(m_pendingVersion);
    });
}

bool LinuxUpdater::prepareRollback(QString* error)
{
    const QString current = QCoreApplication::applicationFilePath();
    if (!current.endsWith(QStringLiteral(".AppImage"), Qt::CaseInsensitive)) {
        return reject(error, QStringLiteral("Only an AppImage install can update itself.")), false;
    }
    m_rollbackPath = QDir::tempPath() + QStringLiteral("/sapie-rollback-%1.AppImage")
        .arg(QCoreApplication::applicationPid());
    if (QFile::exists(m_rollbackPath) && !QFile::remove(m_rollbackPath)) {
        return reject(error, QStringLiteral("An old rollback copy could not be removed.")), false;
    }
    if (!QFile::copy(current, m_rollbackPath)) {
        return reject(error, QStringLiteral("A rollback copy could not be made.")), false;
    }
    return true;
}

bool LinuxUpdater::restoreRollback()
{
    if (m_rollbackPath.isEmpty()) {
        return false;
    }
    const QString current = QCoreApplication::applicationFilePath();
    const bool restored = QFile::rename(m_rollbackPath, current);
    m_rollbackPath.clear();
    return restored;
}

void LinuxUpdater::updateFlatpak()
{
    emit progress(QStringLiteral("flatpak"), 10);
    QProcess flatpak;
    flatpak.start(QStringLiteral("flatpak"),
                  {QStringLiteral("update"), QStringLiteral("-y"), QStringLiteral("org.sapie.SAPIE")});
    if (!flatpak.waitForFinished(120000) || flatpak.exitCode() != 0) {
        emit failed(QStringLiteral("Flatpak could not apply the update. The previous version stays installed."));
        return;
    }
    emit progress(QStringLiteral("flatpak"), 100);
    emit applied(m_pendingVersion);
}

}
