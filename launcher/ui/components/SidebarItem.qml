import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

Control {
    id: root
    property string text: ""
    property string glyph: ""
    property bool selected: false
    property int badge: -1
    property bool demoHover: false
    property bool demoFocus: false
    signal clicked()

    width: parent ? parent.width : 164
    height: 32
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.ListItem
    Accessible.name: root.text
    Accessible.selected: root.selected

    background: Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: UI.Tokens.sp2
        anchors.rightMargin: UI.Tokens.sp2
        radius: UI.Tokens.rXs
        color: root.selected ? UI.Tokens.accentSoft : ((root.hovered || root.demoHover) ? UI.Tokens.bgPanelHover : "transparent")
    }
    contentItem: Row {
        spacing: UI.Tokens.sp2
        leftPadding: UI.Tokens.sp3
        Item {
            width: 14
            height: 14
            Rectangle {
                anchors.fill: parent
                color: "transparent"
                border.color: root.selected ? UI.Tokens.accent : UI.Tokens.borderStrong
                border.width: 1
                radius: UI.Tokens.rXs
            }
            Text {
                anchors.centerIn: parent
                text: root.glyph
                color: root.selected ? UI.Tokens.accent : UI.Tokens.textMuted
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsXs
            }
        }
        Text {
            text: root.text
            color: root.selected ? UI.Tokens.textPrimary : UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
            verticalAlignment: Text.AlignVCenter
        }
        Text {
            visible: root.badge >= 0
            text: root.badge >= 0 ? String(root.badge) : ""
            color: UI.Tokens.accent
            font.family: UI.Tokens.fontMono
            font.pixelSize: UI.Tokens.fsXs
        }
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
