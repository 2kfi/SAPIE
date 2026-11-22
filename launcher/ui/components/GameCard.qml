import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

Control {
    id: root
    property string title: ""
    property url cover: ""
    property int appid: 0
    property bool installed: false
    property bool selected: false
    property bool demoHover: false
    property bool demoFocus: false
    signal clicked()
    signal playRequested()

    width: 148
    height: 272
    focusPolicy: Qt.StrongFocus
    activeFocusOnTab: true
    Accessible.role: Accessible.ListItem
    Accessible.name: root.title + ", " + (root.installed ? "installed" : "not installed")
    Accessible.selected: root.selected

    background: Rectangle {
        anchors.margins: -UI.Tokens.sp1
        radius: UI.Tokens.rSm
        color: (root.hovered || root.demoHover) ? UI.Tokens.bgPanelHover : UI.Tokens.bgPanel
        border.width: root.selected ? 2 : 1
        border.color: root.selected ? UI.Tokens.accent : UI.Tokens.borderSubtle
    }
    contentItem: Column {
        anchors.fill: parent
        anchors.margins: UI.Tokens.sp2
        spacing: UI.Tokens.sp2
        GameArt {
            width: parent.width
            source: root.cover
            label: root.title
            appid: root.appid
        }
        Text {
            width: parent.width
            text: root.title
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
            elide: Text.ElideRight
        }
        StatusPill {
            text: root.installed ? "Installed" : "Not installed"
            status: root.installed ? "online" : "offline"
        }
        PlayButton {
            visible: root.hovered || root.selected || root.demoHover
            gameName: root.title
            onClicked: root.playRequested()
        }
    }
    FocusRing {
        target: root
        cornerRadius: UI.Tokens.rSm
        visible: root.activeFocus || root.demoFocus
    }
    Keys.onReturnPressed: root.clicked()
    Keys.onSpacePressed: root.clicked()
    MouseArea {
        z: -1
        anchors.fill: parent
        onClicked: {
            root.forceActiveFocus()
            root.clicked()
        }
    }
}
