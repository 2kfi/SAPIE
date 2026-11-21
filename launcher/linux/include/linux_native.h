#pragma once

#include <QProcessEnvironment>
#include <QString>
#include <QStringList>

namespace sapie::lnx {

QString dataDirectory();
QString configDirectory();
QString cacheDirectory();
QString runtimeDirectory();
QString tokenFilePath();

bool parseProtocolUrl(const QString& argument, QString* lobbyId, QString* error);
bool registerProtocol(QString* error);
bool registerMimeTypes(const QStringList& mimeTypes, QString* error);

struct LaunchPlan {
    QString program;
    QStringList arguments;
    QProcessEnvironment environment;
};

bool buildLaunchPlan(const QString& executable, const QString& preload,
                     const QStringList& arguments, LaunchPlan* plan, QString* error);
bool startGame(const LaunchPlan& plan, qint64* pid, QString* error);

bool isWayland();
bool isX11();

}
