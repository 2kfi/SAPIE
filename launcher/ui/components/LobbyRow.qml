import QtQuick 2.15
import QtQuick.Controls 2.15
import "." as P
import ".." as UI

Control {
    id: root
    property string name: ""
    property string game: ""
    property int members: -1
    property int capacity: -1
    property string lobbyState: "joinable"
    property bool demoHover: false
    property bool demoFocus: false
    signal join()

    width: parent ? parent.width : 420
    height: 60
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.ListItem
    Accessible.name: root.name + ", " + root.lobbyState

    background: Rectangle {
        radius: UI.Tokens.rXs
        color: (root.hovered || root.demoHover) ? UI.Tokens.bgPanelHover : UI.Tokens.bgPanel
        border.width: 1
        border.color: UI.Tokens.borderSubtle
    }
    contentItem: Row {
        spacing: UI.Tokens.sp3
        leftPadding: UI.Tokens.sp3
        Column {
            width: 180
            anchors.verticalCenter: parent.verticalCenter
            Text {
                text: root.name
                color: UI.Tokens.textPrimary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsSm
                elide: Text.ElideRight
            }
            Text {
                text: root.game
                color: UI.Tokens.textMuted
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsXs
                elide: Text.ElideRight
            }
        }
        P.LobbyBadge { lobbyState: root.lobbyState; anchors.verticalCenter: parent.verticalCenter }
        P.MemberCount {
            members: root.members
            capacity: root.capacity
            anchors.verticalCenter: parent.verticalCenter
        }
        P.Button {
            text: "Join"
            enabled: root.lobbyState === "joinable"
            onClicked: root.join()
            anchors.verticalCenter: parent.verticalCenter
        }
    }
    P.FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
    Keys.onReturnPressed: root.join()
}
