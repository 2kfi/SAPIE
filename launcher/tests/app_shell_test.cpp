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
    void readsInviteFromStartupArguments();
    void forwardsInviteFromSecondInstance();
    void shippedAppWiresNativeCallbacks();
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

void AppShellTest::readsInviteFromStartupArguments()
{
    QCOMPARE(LauncherShell::inviteFromArguments(
                     {QStringLiteral("sapie-launcher"), QStringLiteral("sapie://join/42?exp=1")}),
             QStringLiteral("sapie://join/42?exp=1"));
    QCOMPARE(LauncherShell::inviteFromArguments(
                     {QStringLiteral("sapie-launcher"), QStringLiteral("--no-invite")}),
             QString());
    QCOMPARE(LauncherShell::inviteFromArguments(
                     {QStringLiteral("sapie://join/1\n2")}),
             QString());
}

void AppShellTest::forwardsInviteFromSecondInstance()
{
    LauncherShell primary;
    QVERIFY(primary.acquireSingleInstance());
    QSignalSpy inviteSpy(&primary, &LauncherShell::inviteReceived);

    LauncherShell second;
    QVERIFY(!second.acquireSingleInstance(QStringLiteral("sapie://join/123")));
    QVERIFY(inviteSpy.wait(2000));
    QCOMPARE(inviteSpy.count(), 1);
    QCOMPARE(inviteSpy.first().at(0).toString(), QStringLiteral("sapie://join/123"));
}

void AppShellTest::shippedAppWiresNativeCallbacks()
{
    QFile source(QStringLiteral(SAPIE_MAIN_SOURCE));
    QVERIFY(source.open(QIODevice::ReadOnly));
    const QString main = QString::fromUtf8(source.readAll());
    QVERIFY(main.contains(QStringLiteral("QApplication app")));
    QVERIFY(main.contains(QStringLiteral("inviteFromArguments")));
    QVERIFY(main.contains(QStringLiteral("pickExecutableRequested")));
    QVERIFY(main.contains(QStringLiteral("setChosenExecutable(path)")));
    QVERIFY(main.contains(QStringLiteral("inviteReceived")));
    QVERIFY(main.contains(QStringLiteral("core.handleDeepLink(url)")));
    QVERIFY(!main.contains(QStringLiteral("QLocalServer")));
}

QTEST_MAIN(AppShellTest)
#include "app_shell_test.moc"
