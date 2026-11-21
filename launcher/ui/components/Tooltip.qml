import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string text: ""
    property bool shown: false

    implicitWidth: label.implicitWidth + UI.Tokens.sp4
    implicitHeight: 28
    visible: root.shown
    Accessible.role: Accessible.ToolTip
    Accessible.name: root.text

    Rectangle {
        anchors.fill: parent
        radius: UI.Tokens.rXs
        color: UI.Tokens.bgElevated
        border.width: 1
        border.color: UI.Tokens.borderStrong
    }
    Text {
        id: label
        anchors.fill: parent
        anchors.margins: UI.Tokens.sp2
        text: root.text
        color: UI.Tokens.textPrimary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsXs
        verticalAlignment: Text.AlignVCenter
    }
}
