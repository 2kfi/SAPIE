import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string lobbyState: "joinable"

    implicitWidth: dot.width + UI.Tokens.sp2 + label.implicitWidth + UI.Tokens.sp2
    implicitHeight: 22
    Accessible.role: Accessible.Indicator
    Accessible.name: root.lobbyState

    function tone() {
        if (root.lobbyState === "joinable") return UI.Tokens.success
        if (root.lobbyState === "full") return UI.Tokens.warning
        return UI.Tokens.textMuted
    }
    function title() {
        if (root.lobbyState === "joinable") return "Joinable"
        if (root.lobbyState === "full") return "Full"
        if (root.lobbyState === "private") return "Private"
        return "Unknown"
    }
    Rectangle {
        id: dot
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        width: 8
        height: 8
        radius: width / 2
        color: root.tone()
    }
    Text {
        id: label
        anchors.left: dot.right
        anchors.leftMargin: UI.Tokens.sp2
        anchors.verticalCenter: parent.verticalCenter
        text: root.title()
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsXs
    }
}
