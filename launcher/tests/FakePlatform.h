#pragma once

#include "Platform.h"

#include <QPair>
#include <QStringList>
#include <QVector>

class FakePlatform final : public Platform
{
public:
    QString chooseExecutable(QObject *parent) override
    {
        Q_UNUSED(parent)
        return executable;
    }

    void openFolder(const QString &path) override
    {
        openedFolders.append(path);
    }

    bool registerProtocolHandler() override
    {
        protocolRegistered = true;
        return true;
    }

    bool enableSingleInstance() override
    {
        singleInstance = true;
        return true;
    }

    void showTrayMessage(const QString &title, const QString &message) override
    {
        trayMessages.append(qMakePair(title, message));
    }

    QString executable;
    QStringList openedFolders;
    QVector<QPair<QString, QString>> trayMessages;
    bool protocolRegistered = false;
    bool singleInstance = false;
};
