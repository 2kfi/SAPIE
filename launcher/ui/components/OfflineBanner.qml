import QtQuick 2.15
import ".." as UI

Rectangle {
    id: root
    property string message: "Server offline. Cached config is still available."

    height: 32
    radius: UI.Tokens.rXs
    color: UI.Tokens.bgPanel
    border.width: 1
    border.color: UI.Tokens.warning

    Accessible.role: Accessible.StatusBar
    Accessible.name: root.message

    Rectangle {
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp3
        anchors.verticalCenter: parent.verticalCenter
        width: 8
        height: 8
        radius: width / 2
        color: UI.Tokens.warning
    }
    Text {
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp4 + UI.Tokens.sp2
        anchors.verticalCenter: parent.verticalCenter
        text: root.message
        color: UI.Tokens.textPrimary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsXs
    }
}
