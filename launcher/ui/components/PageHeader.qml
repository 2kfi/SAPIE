import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string title: ""
    property string subtitle: ""
    default property alias actions: actionsRow.data

    height: 64
    Accessible.role: Accessible.Heading
    Accessible.name: root.title

    Column {
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.leftMargin: UI.Tokens.sp4
        anchors.topMargin: UI.Tokens.sp3
        spacing: UI.Tokens.sp1
        Text {
            text: root.title
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsXl
            font.weight: Font.DemiBold
        }
        Text {
            visible: root.subtitle !== ""
            text: root.subtitle
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
        }
    }
    Row {
        id: actionsRow
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.rightMargin: UI.Tokens.sp4
        spacing: UI.Tokens.sp2
    }
}
