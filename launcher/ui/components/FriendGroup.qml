import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string title: ""
    default property alias rows: column.data

    Accessible.role: Accessible.Grouping
    Accessible.name: root.title

    Text {
        id: heading
        text: root.title
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
    }
    Column {
        id: column
        anchors.top: heading.bottom
        anchors.topMargin: UI.Tokens.sp2
        anchors.left: parent.left
        anchors.right: parent.right
        spacing: UI.Tokens.sp1
    }
    height: heading.height + UI.Tokens.sp2 + column.implicitHeight
}
