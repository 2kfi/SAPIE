import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string title: ""
    default property alias content: body.data

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
        id: body
        anchors.top: heading.bottom
        anchors.topMargin: UI.Tokens.sp2
        anchors.left: parent.left
        anchors.right: parent.right
        spacing: UI.Tokens.sp2
    }
    height: heading.height + UI.Tokens.sp2 + body.implicitHeight
}
