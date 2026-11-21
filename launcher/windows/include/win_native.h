#pragma once

#include <QString>
#include <QStringList>

class QWindow;

namespace sapie::win {

QString dataDirectory();
QString extendedLengthPath(const QString& path);

bool parseProtocolUrl(const QString& argument, QString* lobbyId, QString* error);

bool registerProtocol(QString* error);
bool unregisterProtocol(QString* error);
bool registerFileAssociations(const QStringList& extensions, QString* error);
bool unregisterFileAssociations(const QStringList& extensions, QString* error);

void applyDarkTitleBar(QWindow* window);
void setAppUserModelId();

}
