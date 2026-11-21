import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string title: ""
    default property alias items: rows.data

    Accessible.role: Accessible.Grouping
    Accessible.name: root.title

    implicitHeight: label.height + rows.implicitHeight + UI.Tokens.sp2
    Text {
        id: label
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp3
        text: root.title.toUpperCase()
        color: UI.Tokens.textMuted
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsXs
    }
    Column {
        id: rows
        anchors.top: label.bottom
        anchors.topMargin: UI.Tokens.sp2
        anchors.left: parent.left
        anchors.right: parent.right
        spacing: UI.Tokens.sp1
    }
}
