#include "CoreBridge.h"

#include <QCoreApplication>
#include <QGuiApplication>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QUrl>

int main(int argc, char *argv[])
{
    QGuiApplication app(argc, argv);
    app.setOrganizationName("SAPIE");
    app.setApplicationName("SAPIE Launcher");

    CoreBridge core;
    if (!core.createCore(core.abiVersion(), false)) {
        qWarning() << core.lastError();
    }

    QQmlApplicationEngine engine;
    engine.rootContext()->setContextProperty("SapieCore", &core);
    QObject::connect(&engine, &QQmlApplicationEngine::objectCreationFailed, &app,
                     []() { QCoreApplication::exit(1); }, Qt::QueuedConnection);
    engine.load(QUrl(QStringLiteral("qrc:/main.qml")));

    return app.exec();
}
