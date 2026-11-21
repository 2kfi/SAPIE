import QtQuick 2.15
import QtQuick.Window 2.15
import SAPIE.UI 1.0

Window {
    id: root
    width: 960
    height: 640
    minimumWidth: 640
    minimumHeight: 480
    visible: true
    title: "SAPIE"
    color: Tokens.bgApp

    FontLoader { source: "qrc:/Manrope.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-Regular.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-Medium.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-SemiBold.ttf" }
    FontLoader { source: "qrc:/IBMPlexMono-Bold.ttf" }

    Rectangle {
        id: mark
        width: 28
        height: 28
        radius: 6
        color: Tokens.accentSoft
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: wordmark.top
        anchors.bottomMargin: 24

        Rectangle {
            width: 10
            height: 10
            radius: 3
            color: Tokens.accent
            anchors.centerIn: parent
        }
    }

    Text {
        id: wordmark
        anchors.centerIn: parent
        text: "SAPIE"
        color: Tokens.textPrimary
        font.family: "Manrope"
        font.pixelSize: 42
        font.weight: Font.Bold
    }

    Text {
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.top: wordmark.bottom
        anchors.topMargin: 10
        text: "Local game launcher"
        color: Tokens.textSecondary
        font.family: "Manrope"
        font.pixelSize: 15
        font.weight: Font.Medium
    }
}
