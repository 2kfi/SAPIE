#pragma once

#include <QByteArray>
#include <QObject>
#include <QString>
#include <QUrl>
#include <QVector>

class QNetworkAccessManager;

namespace sapie::win {

struct UpdateArtifact {
    QString name;
    QString os;
    QString arch;
    QString sha256;
    qlonglong size = 0;
    QUrl url;
};

struct UpdateManifest {
    QString schema;
    QString channel;
    QString version;
    QString tag;
    QString previousVersion;
    QString rollback;
    QVector<UpdateArtifact> artifacts;
    QByteArray body;
};

bool verifyManifestSignature(const QByteArray& body,
                             const QByteArray& signatureBase64,
                             const QString& publicKeyPemPath,
                             QString* error);

class WindowsUpdater : public QObject {
    Q_OBJECT

public:
    WindowsUpdater(const QString& channel,
                   const QString& arch,
                   const QString& publicKeyPemPath,
                   QObject* parent = nullptr);

    void check(const QUrl& manifestUrl, const QString& currentVersion);
    void apply(const UpdateManifest& manifest, const QString& currentVersion);

signals:
    void updateAvailable(const UpdateManifest& manifest);
    void upToDate();
    void failed(const QString& message);
    void progress(const QString& stage, int percent);
    void applied(const QString& version);

private slots:
    void onManifestReceived();
    void onSignatureReceived();
    void onArtifactDownloaded();

private:
    UpdateManifest parseManifest(const QByteArray& body, QString* error) const;
    bool selectArtifact(const UpdateManifest& manifest, UpdateArtifact* artifact, QString* error) const;
    bool validateArtifact(const QByteArray& bytes, const UpdateArtifact& artifact, QString* error) const;
    bool verifyAuthenticode(const QString& path, QString* error) const;
    bool prepareRollback(const QString& currentVersion, QString* error);
    bool restoreRollback(QString* error);

    QNetworkAccessManager* m_network = nullptr;
    QString m_channel;
    QString m_arch;
    QString m_publicKeyPemPath;
    QString m_currentVersion;
    QString m_pendingVersion;
    QByteArray m_manifestBody;
    QUrl m_manifestUrl;
    UpdateArtifact m_pendingArtifact;
    QString m_pendingPath;
    QString m_rollbackDir;
};

}
