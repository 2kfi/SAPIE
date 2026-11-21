import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property bool open: false
    property string label: "Menu"
    property int focusIndex: 0
    default property alias items: column.data

    implicitWidth: 168
    implicitHeight: column.implicitHeight
    visible: root.open
    focus: root.open
    Accessible.role: Accessible.PopupMenu
    Accessible.name: root.label

    function focusAt(index) {
        var list = column.children
        if (list.length === 0) return
        root.focusIndex = (index + list.length) % list.length
        list[root.focusIndex].forceActiveFocus()
    }
    function cycle(step) {
        root.focusAt(root.focusIndex + step)
    }
    onOpenChanged: if (root.open) Qt.callLater(root.focusAt, 0)

    Keys.onEscapePressed: root.open = false
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
        radius: UI.Tokens.rSm
        color: UI.Tokens.bgElevated
        border.width: 1
        border.color: UI.Tokens.borderStrong
    }
    Column {
        id: column
        anchors.fill: parent
        anchors.margins: UI.Tokens.sp1
        spacing: UI.Tokens.sp1
    }
}
