#pragma once

#include <QByteArray>
#include <QString>

namespace sapie::win {

bool writeOwnerOnlyFile(const QString& path, const QByteArray& contents, QString* error);
bool verifyOwnerOnlyFile(const QString& path, QString* error);

}
