import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property int appid: 0
    property string label: ""
    readonly property int variant: Math.abs(root.appid) % 4
    readonly property var fills: [UI.Tokens.accentSoft, UI.Tokens.bgElevated, UI.Tokens.bgPanelHover, UI.Tokens.borderStrong]

    Accessible.role: Accessible.Graphic
    Accessible.name: root.label + ", monogram for appid " + root.appid

    Rectangle {
        anchors.fill: parent
        radius: UI.Tokens.rXs
        color: root.fills[root.variant]
        border.width: 1
        border.color: UI.Tokens.borderSubtle
    }
    Rectangle {
        x: parent.width * (0.18 + 0.12 * (root.variant % 2))
        y: parent.height * 0.2
        width: parent.width * 0.64
        height: 2
        color: UI.Tokens.accent
    }
    Rectangle {
        x: parent.width * (root.variant < 2 ? 0.24 : 0.36)
        y: parent.height * 0.34
        width: parent.width * 0.4
        height: parent.height * 0.32
        radius: root.variant % 2 === 0 ? UI.Tokens.rSm : UI.Tokens.rXs
        color: "transparent"
        border.width: 1
        border.color: UI.Tokens.textSecondary
    }
    Text {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.margins: UI.Tokens.sp2
        text: root.label
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
        horizontalAlignment: Text.AlignHCenter
        elide: Text.ElideRight
    }
}
