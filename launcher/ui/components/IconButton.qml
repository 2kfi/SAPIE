import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

Control {
    id: root
    property string glyph: ""
    property string label: ""
    property string variant: "quiet"
    property bool demoHover: false
    property bool demoFocus: false
    signal clicked()

    width: 32
    height: 32
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.Button
    Accessible.name: root.label

    background: Rectangle {
        anchors.margins: 2
        radius: UI.Tokens.rXs
        color: (root.hovered || root.demoHover) ? UI.Tokens.bgPanelHover : "transparent"
        border.width: 1
        border.color: root.variant === "danger" ? UI.Tokens.danger : UI.Tokens.borderSubtle
    }
    contentItem: Text {
        text: root.glyph
        color: root.variant === "danger" ? UI.Tokens.danger : UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsMd
        horizontalAlignment: Text.AlignHCenter
        verticalAlignment: Text.AlignVCenter
    }
    FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
    Keys.onReturnPressed: root.clicked()
    Keys.onSpacePressed: root.clicked()
    MouseArea {
        anchors.fill: parent
        onClicked: {
            root.forceActiveFocus()
            root.clicked()
        }
    }
}
