import QtQuick 2.15
import QtQuick.Window 2.15
import ".." as UI

Window {
    id: window
    visible: true
    width: 1280
    height: 720
    minimumWidth: 860
    minimumHeight: 560
    color: UI.Tokens.bgApp
    title: "SAPIE launcher QML test harness, test data"

    LauncherApp {
        anchors.fill: parent
        bridge: UI.FakeCoreBridge
        Component.onCompleted: {
            UI.FakeCoreBridge.connectionState = "signedIn"
            UI.FakeCoreBridge.firstRun = false
        }
    }
}
