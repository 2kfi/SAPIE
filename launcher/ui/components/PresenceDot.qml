import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string status: "offline"

    width: 8
    height: 8
    Accessible.role: Accessible.Indicator
    Accessible.name: root.status + " presence dot"

    function tone() {
        if (root.status === "online") return UI.Tokens.success
        if (root.status === "away" || root.status === "busy") return UI.Tokens.warning
        if (root.status === "error") return UI.Tokens.danger
        return UI.Tokens.textMuted
    }
    Rectangle {
        anchors.fill: parent
        radius: width / 2
        color: root.tone()
    }
}
