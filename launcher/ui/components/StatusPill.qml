import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string text: ""
    property string status: "info"

    implicitWidth: dot.width + UI.Tokens.sp2 + label.implicitWidth + UI.Tokens.sp3
    implicitHeight: 22
    Accessible.role: Accessible.Indicator
    Accessible.name: root.text
    Accessible.description: root.status

    function tone() {
        if (root.status === "online" || root.status === "success") return UI.Tokens.success
        if (root.status === "away" || root.status === "warning") return UI.Tokens.warning
        if (root.status === "offline" || root.status === "danger" || root.status === "error") return UI.Tokens.danger
        return UI.Tokens.info
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
        text: root.text
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsXs
    }
}
