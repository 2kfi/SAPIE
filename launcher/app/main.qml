import QtQuick 2.15
import QtQuick.Window 2.15
import SAPIE.UI 1.0

Window {
    id: root
    width: 960
    height: 640
    minimumWidth: 860
    minimumHeight: 560
    visible: true
    title: "SAPIE"
    color: Tokens.bgApp

    FontLoader { source: "qrc:/Manrope.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-Regular.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-Medium.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-SemiBold.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-Bold.ttf" }

    LauncherApp {
        anchors.fill: parent
    }
}
