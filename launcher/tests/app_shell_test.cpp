#include "LauncherShell.h"

#include <QColor>
#include <QFile>
#include <QQuickWindow>
#include <QQmlApplicationEngine>
#include <QSignalSpy>
#include <QTest>
#include <QUrl>

class AppShellTest : public QObject
{
    Q_OBJECT

private slots:
    void redactsCredentialValues();
    void loadsTheShellFromResources();
};

void AppShellTest::redactsCredentialValues()
{
    QCOMPARE(LauncherShell::redact(QStringLiteral("password=hunter2 token=abc")),
             QStringLiteral("password=[redacted] token=[redacted]"));
    QCOMPARE(LauncherShell::redact(QStringLiteral("{\"password\":\"plain\"}")),
             QStringLiteral("{\"password\":\"[redacted]\"}"));
}

void AppShellTest::loadsTheShellFromResources()
{
    QFile qml(QStringLiteral(":/main.qml"));
    QVERIFY(qml.open(QIODevice::ReadOnly));
    const QString source = QString::fromUtf8(qml.readAll());
    QVERIFY(!source.contains(QStringLiteral("http://")));
    QVERIFY(!source.contains(QStringLiteral("https://")));
    QVERIFY(QFile::exists(QStringLiteral(":/Manrope.ttf")));
    QVERIFY(QFile::exists(QStringLiteral(":/qt/qml/SAPIE/UI/qmldir")));

    QQmlApplicationEngine engine;
    engine.addImportPath(QStringLiteral("qrc:/qt/qml"));
    QSignalSpy creationFailed(&engine, &QQmlApplicationEngine::objectCreationFailed);
    engine.load(QUrl(QStringLiteral("qrc:/main.qml")));
    QCOMPARE(creationFailed.count(), 0);
    QCOMPARE(engine.rootObjects().size(), 1);

    auto *window = qobject_cast<QQuickWindow *>(engine.rootObjects().first());
    QVERIFY(window != nullptr);
    QCOMPARE(window->color(), QColor(QStringLiteral("#17191D")));
}

QTEST_MAIN(AppShellTest)
#include "app_shell_test.moc"
