import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property bool open: false
    property string title: ""
    property string side: "right"
    property Item trigger: null
    default property alias content: body.data

    anchors.fill: parent
    visible: root.open
    focus: root.open
    Accessible.role: Accessible.Pane
    Accessible.name: root.title

    function close() {
        root.open = false
        if (root.trigger) root.trigger.forceActiveFocus()
    }
    Keys.onEscapePressed: root.close()

    Rectangle {
        anchors.fill: parent
        color: UI.Tokens.bgApp
        opacity: 0.72
        MouseArea {
            anchors.fill: parent
            onClicked: root.close()
        }
    }
    Rectangle {
        width: Math.min(320, parent.width / 2)
        height: parent.height
        anchors.top: parent.top
        anchors.left: root.side === "left" ? parent.left : undefined
        anchors.right: root.side === "left" ? undefined : parent.right
        color: UI.Tokens.bgElevated
        border.width: 1
        border.color: UI.Tokens.borderSubtle
        Column {
            id: body
            anchors.fill: parent
            anchors.margins: UI.Tokens.sp4
            spacing: UI.Tokens.sp3
        }
    }
}
