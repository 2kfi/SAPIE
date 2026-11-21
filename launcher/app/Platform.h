#pragma once

#include <QObject>
#include <QString>

class Platform
{
public:
    virtual ~Platform() = default;

    virtual QString chooseExecutable(QObject *parent) = 0;
    virtual void openFolder(const QString &path) = 0;
    virtual bool registerProtocolHandler() = 0;
    virtual bool enableSingleInstance() = 0;
    virtual void showTrayMessage(const QString &title, const QString &message) = 0;
};
