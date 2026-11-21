#include "CoreBridge.h"
#include "LauncherShell.h"

#include <QCoreApplication>
#include <QGuiApplication>
#include <QPointer>
#include <QQmlApplicationEngine>
#include <QQmlContext>
#include <QQuickWindow>
#include <QUrl>

int main(int argc, char *argv[])
{
    installLauncherLogging();

    QGuiApplication app(argc, argv);
    app.setOrganizationName(QStringLiteral("SAPIE"));
    app.setApplicationName(QStringLiteral("SAPIE Launcher"));

    LauncherShell shell;
    if (!shell.acquireSingleInstance()) {
        return 0;
    }

    CoreBridge core;
    if (!core.createCore(core.abiVersion(), false)) {
        qWarning() << core.lastError();
    }

    QQmlApplicationEngine engine;
    engine.addImportPath(QStringLiteral("qrc:/qt/qml"));
    engine.rootContext()->setContextProperty(QStringLiteral("SapieCore"), &core);
    QObject::connect(&engine, &QQmlApplicationEngine::objectCreationFailed, &app,
                     []() { QCoreApplication::exit(1); }, Qt::QueuedConnection);
    engine.load(QUrl(QStringLiteral("qrc:/main.qml")));
    if (engine.rootObjects().isEmpty()) {
        return 1;
    }

    QPointer<QQuickWindow> window = qobject_cast<QQuickWindow *>(engine.rootObjects().first());
    shell.restoreWindow(window);
    QObject::connect(&app, &QGuiApplication::aboutToQuit, &app, [&shell, &core, window]() {
        shell.saveWindow(window);
        core.shutdown();
    });

    return app.exec();
}
