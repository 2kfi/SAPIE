import QtQuick 2.15
import QtQuick.Controls 2.15
import "." as P
import ".." as UI

Control {
    id: root
    property string name: ""
    property bool demoHover: false
    property bool demoFocus: false
    signal unblocked()

    width: parent ? parent.width : 300
    height: 52
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.ListItem
    Accessible.name: root.name + ", blocked"

    background: Rectangle {
        radius: UI.Tokens.rXs
        color: (root.hovered || root.demoHover) ? UI.Tokens.bgPanelHover : "transparent"
    }
    contentItem: Row {
        spacing: UI.Tokens.sp3
        leftPadding: UI.Tokens.sp2
        P.Avatar { name: root.name; size: 32; anchors.verticalCenter: parent.verticalCenter }
        Text {
            width: 140
            anchors.verticalCenter: parent.verticalCenter
            text: root.name
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
            elide: Text.ElideRight
        }
        P.Button {
            text: "Unblock"
            variant: "quiet"
            onClicked: root.unblocked()
        }
    }
    P.FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
}
