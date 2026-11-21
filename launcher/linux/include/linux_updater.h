#pragma once

#include <QByteArray>
#include <QObject>
#include <QString>
#include <QUrl>
#include <QVector>

class QNetworkAccessManager;
class QNetworkReply;

namespace sapie::lnx {

struct UpdateArtifact {
    QString name;
    QString os;
    QString arch;
    QString sha256;
    qint64 size = 0;
    QUrl url;
};

struct UpdateManifest {
    QString schema;
    QString channel;
    QString version;
    QString previousVersion;
    QString rollback;
    QVector<UpdateArtifact> artifacts;
    QByteArray body;
};

bool parseUpdateManifest(const QByteArray& body, const QString& channel,
                         UpdateManifest* out, QString* error);
bool verifyManifestSignature(const QByteArray& body, const QByteArray& signatureBase64,
                             const QString& publicKeyPemPath, QString* error);

class LinuxUpdater : public QObject {
    Q_OBJECT

public:
    LinuxUpdater(const QString& channel, const QString& arch, const QString& kind,
                 const QString& publicKeyPemPath, QObject* parent = nullptr);
    ~LinuxUpdater() override;

    void check(const QUrl& manifestUrl, const QString& currentVersion);
    void apply(const UpdateManifest& manifest, const QString& currentVersion);

signals:
    void updateAvailable(const UpdateManifest& manifest);
    void upToDate();
    void failed(const QString& message);
    void progress(const QString& stage, int percent);
    void applied(const QString& version);

private slots:
    void onSignatureReceived();

private:
    bool selectArtifact(const UpdateManifest& manifest, UpdateArtifact* artifact,
                        QString* error) const;
    bool prepareRollback(QString* error);
    bool restoreRollback();
    void applyAppImage(const UpdateArtifact& artifact);
    void updateFlatpak();

    QNetworkAccessManager* m_network = nullptr;
    QString m_channel;
    QString m_arch;
    QString m_kind;
    QString m_publicKeyPemPath;
    QString m_currentVersion;
    QString m_pendingVersion;
    QByteArray m_manifestBody;
    UpdateArtifact m_pendingArtifact;
    QString m_rollbackPath;
};

}
