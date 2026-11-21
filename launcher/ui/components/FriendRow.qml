import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

Control {
    id: root
    property string name: ""
    property string status: "offline"
    property string note: ""
    property bool selected: false
    property bool demoHover: false
    property bool demoFocus: false
    signal clicked()

    width: parent ? parent.width : 280
    height: 52
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.ListItem
    Accessible.name: root.name + ", " + root.status
    Accessible.selected: root.selected

    background: Rectangle {
        color: root.selected ? UI.Tokens.accentSoft : ((root.hovered || root.demoHover) ? UI.Tokens.bgPanelHover : "transparent")
        radius: UI.Tokens.rXs
    }
    contentItem: Row {
        spacing: UI.Tokens.sp3
        leftPadding: UI.Tokens.sp2
        Avatar { name: root.name; size: 36; anchors.verticalCenter: parent.verticalCenter }
        Column {
            anchors.verticalCenter: parent.verticalCenter
            spacing: UI.Tokens.sp1
            Text {
                text: root.name
                color: UI.Tokens.textPrimary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsSm
            }
            Row {
                spacing: UI.Tokens.sp2
                PresenceDot { status: root.status; anchors.verticalCenter: parent.verticalCenter }
                Text {
                    text: root.status
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsXs
                }
                Text {
                    visible: root.note !== ""
                    text: root.note
                    color: UI.Tokens.textMuted
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsXs
                }
            }
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
