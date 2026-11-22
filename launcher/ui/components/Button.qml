import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

Control {
    id: root
    property string text: ""
    property string variant: "primary"
    property bool loading: false
    property bool demoHover: false
    property bool demoFocus: false
    signal clicked()

    width: 104
    height: 32
    padding: UI.Tokens.sp2
    enabled: !root.loading
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.Button
    Accessible.name: root.text

    property bool danger: root.variant === "danger"
    property bool quiet: root.variant === "quiet"
    property bool highlighted: root.hovered || root.demoHover

    background: Rectangle {
        opacity: root.highlighted && root.variant === "primary" ? 0.86 : 1
        radius: UI.Tokens.rSm
        color: {
            if (root.danger) return root.highlighted ? UI.Tokens.bgElevated : UI.Tokens.bgPanel
            if (root.quiet) return root.highlighted ? UI.Tokens.bgPanelHover : "transparent"
            return UI.Tokens.accent
        }
        border.width: 1
        border.color: root.danger ? UI.Tokens.danger : (root.quiet ? (root.highlighted ? UI.Tokens.borderStrong : UI.Tokens.borderSubtle) : UI.Tokens.accent)
    }
    contentItem: Text {
        text: root.loading ? "Working..." : root.text
        color: root.danger ? UI.Tokens.danger : (root.quiet ? UI.Tokens.textPrimary : UI.Tokens.accentInk)
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
        elide: Text.ElideRight
    }
    FocusRing { target: root; cornerRadius: UI.Tokens.rSm; visible: root.activeFocus || root.demoFocus }

    Keys.onReturnPressed: root.clicked()
    Keys.onSpacePressed: root.clicked()
    MouseArea {
        anchors.fill: parent
        cursorShape: Qt.PointingHandCursor
        enabled: root.enabled
        onClicked: root.clicked()
    }
}
