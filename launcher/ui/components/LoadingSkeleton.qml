pragma ComponentBehavior: Bound
import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property int lines: 3
    property bool animate: true

    implicitHeight: column.implicitHeight
    Accessible.role: Accessible.ProgressBar
    Accessible.name: "Loading"

    Column {
        id: column
        anchors.fill: parent
        spacing: UI.Tokens.sp2
        Repeater {
            model: root.lines
            Rectangle {
                required property int index
                width: index === root.lines - 1 ? parent.width * 0.6 : parent.width
                height: 12
                radius: UI.Tokens.rXs
                color: UI.Tokens.bgPanelHover
                opacity: root.animate ? (0.5 + 0.2 * Math.sin(index + 1)) : 0.6
                Accessible.ignored: true
            }
        }
    }
}
