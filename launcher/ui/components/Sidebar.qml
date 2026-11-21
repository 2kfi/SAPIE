import QtQuick 2.15
import ".." as UI

Rectangle {
    id: root
    width: 164
    color: UI.Tokens.bgSidebar
    default property alias items: column.data

    Accessible.role: Accessible.Pane
    Accessible.name: "Navigation"

    Column {
        id: column
        anchors.fill: parent
        anchors.topMargin: UI.Tokens.sp3
        spacing: UI.Tokens.sp4
    }
    Rectangle {
        anchors.top: parent.top
        anchors.bottom: parent.bottom
        anchors.right: parent.right
        width: 1
        color: UI.Tokens.borderSubtle
    }
}
