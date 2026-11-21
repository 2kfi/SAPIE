import QtQuick 2.15
import ".." as UI

Rectangle {
    id: ring
    property Item target: parent
    property int cornerRadius: UI.Tokens.rSm

    anchors.fill: target
    anchors.margins: -2
    z: 2
    radius: ring.cornerRadius + 2
    color: "transparent"
    border.color: UI.Tokens.accent
    border.width: 2
    visible: target && target.activeFocus
}
