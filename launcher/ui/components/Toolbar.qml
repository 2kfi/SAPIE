import QtQuick 2.15
import ".." as UI

Rectangle {
    id: root
    height: 44
    color: UI.Tokens.bgApp
    default property alias content: row.data

    Accessible.role: Accessible.ToolBar
    Accessible.name: "Toolbar"

    Row {
        id: row
        anchors.fill: parent
        anchors.leftMargin: UI.Tokens.sp3
        anchors.rightMargin: UI.Tokens.sp3
        spacing: UI.Tokens.sp2
    }
    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 1
        color: UI.Tokens.borderSubtle
    }
}
