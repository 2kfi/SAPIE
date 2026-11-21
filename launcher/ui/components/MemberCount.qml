import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property int members: -1
    property int capacity: -1
    readonly property bool known: root.members >= 0 && root.capacity >= 0

    implicitWidth: label.implicitWidth
    implicitHeight: 18
    Accessible.role: Accessible.StaticText
    Accessible.name: root.known ? (root.members + " of " + root.capacity + " members") : "Member count unavailable"

    Text {
        id: label
        anchors.fill: parent
        text: root.known ? (root.members + " / " + root.capacity) : "-- / --"
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontMono
        font.pixelSize: UI.Tokens.fsXs
        verticalAlignment: Text.AlignVCenter
    }
}
