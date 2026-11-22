import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string message: ""
    property bool shown: true
    property int duration: 4000

    height: 40
    visible: root.shown
    Accessible.role: Accessible.Notification
    Accessible.name: root.message

    Rectangle {
        anchors.fill: parent
        radius: UI.Tokens.rSm
        color: UI.Tokens.bgElevated
        border.width: 1
        border.color: UI.Tokens.borderStrong
    }
    Text {
        anchors.fill: parent
        anchors.margins: UI.Tokens.sp3
        text: root.message
        color: UI.Tokens.textPrimary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
        verticalAlignment: Text.AlignVCenter
    }
    Timer {
        running: root.shown && root.duration > 0
        interval: root.duration
        onTriggered: root.shown = false
    }
}
