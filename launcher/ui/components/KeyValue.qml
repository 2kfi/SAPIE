import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string keyName: ""
    property string value: ""

    height: 22
    Accessible.role: Accessible.StaticText
    Accessible.name: root.keyName + ": " + root.value

    Text {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        text: root.keyName
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
    }
    Text {
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        text: root.value
        color: UI.Tokens.textPrimary
        font.family: UI.Tokens.fontMono
        font.pixelSize: UI.Tokens.fsSm
    }
}
