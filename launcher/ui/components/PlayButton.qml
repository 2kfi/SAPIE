import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

Control {
    id: root
    property string gameName: ""
    property bool launching: false
    property bool demoHover: false
    property bool demoFocus: false
    signal clicked()

    width: 76
    height: 28
    enabled: !root.launching
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.Button
    Accessible.name: "Play " + root.gameName

    background: Rectangle {
        radius: UI.Tokens.rXs
        color: UI.Tokens.accent
        opacity: (root.hovered || root.demoHover) ? 0.86 : 1
    }
    contentItem: Row {
        spacing: UI.Tokens.sp1
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: "\u25B6"
            color: UI.Tokens.accentInk
            font.pixelSize: UI.Tokens.fsXs
        }
        Text {
            anchors.verticalCenter: parent.verticalCenter
            text: root.launching ? "Launching" : "Play"
            color: UI.Tokens.accentInk
            font.family: UI.Tokens.fontUi
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
