import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property bool open: false
    property string title: ""
    property Item trigger: null
    property var focusTargets: []
    property int focusIndex: 0
    default property alias content: body.data
    property alias actions: actions.data
    signal accepted()
    signal rejected()

    anchors.fill: parent
    visible: root.open
    focus: root.open
    Accessible.role: Accessible.Dialog
    Accessible.name: root.title

    function close() {
        root.open = false
        if (root.trigger) root.trigger.forceActiveFocus()
        root.rejected()
    }
    function focusAt(index) {
        if (root.focusTargets.length === 0) return
        root.focusIndex = (index + root.focusTargets.length) % root.focusTargets.length
        root.focusTargets[root.focusIndex].forceActiveFocus()
    }
    function cycle(step) {
        root.focusAt(root.focusIndex + step)
    }
    onOpenChanged: if (root.open) Qt.callLater(root.focusAt, 0)

    Keys.onEscapePressed: root.close()
    Shortcut {
        sequence: "Tab"
        context: Qt.WindowShortcut
        enabled: root.open
        onActivated: root.cycle(1)
    }
    Shortcut {
        sequence: "Shift+Tab"
        context: Qt.WindowShortcut
        enabled: root.open
        onActivated: root.cycle(-1)
    }

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
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        height: panelContent.implicitHeight + UI.Tokens.sp8
        anchors.centerIn: parent
        radius: UI.Tokens.rLg
        color: UI.Tokens.bgElevated
        border.width: 1
        border.color: UI.Tokens.borderStrong
        MouseArea {
            anchors.fill: parent
            onClicked: function(mouse) { mouse.accepted = false }
        }
        Column {
            id: panelContent
            anchors.top: parent.top
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.margins: UI.Tokens.sp4
            spacing: UI.Tokens.sp4
            Text {
                text: root.title
                color: UI.Tokens.textPrimary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsLg
            }
            Column {
                id: body
                width: parent.width
            }
            Row {
                id: actions
                anchors.right: parent.right
                spacing: UI.Tokens.sp2
            }
        }
    }
}
