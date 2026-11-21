import QtQuick 2.15
import QtQuick.Controls 2.15
import "." as P
import ".." as UI

Control {
    id: root
    property string name: ""
    property string direction: "incoming"
    property bool demoHover: false
    property bool demoFocus: false
    signal accepted()
    signal declined()

    width: parent ? parent.width : 320
    height: 56
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.ListItem
    Accessible.name: root.name + ", " + root.direction + " request"

    background: Rectangle {
        radius: UI.Tokens.rXs
        color: (root.hovered || root.demoHover) ? UI.Tokens.bgPanelHover : UI.Tokens.bgPanel
    }
    contentItem: Row {
        spacing: UI.Tokens.sp2
        leftPadding: UI.Tokens.sp2
        P.Avatar { name: root.name; size: 32; anchors.verticalCenter: parent.verticalCenter }
        Column {
            width: 120
            anchors.verticalCenter: parent.verticalCenter
            Text {
                text: root.name
                color: UI.Tokens.textPrimary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsSm
                elide: Text.ElideRight
            }
            Text {
                text: root.direction === "incoming" ? "Incoming request" : "Outgoing request"
                color: UI.Tokens.textMuted
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsXs
            }
        }
        P.Button {
            visible: root.direction === "incoming"
            text: "Accept"
            onClicked: root.accepted()
        }
        P.Button {
            text: root.direction === "incoming" ? "Decline" : "Cancel"
            variant: "quiet"
            onClicked: root.declined()
        }
    }
    P.FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
    Keys.onReturnPressed: root.accepted()
}
