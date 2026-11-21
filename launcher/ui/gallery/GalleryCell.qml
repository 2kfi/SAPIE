import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string caption: ""
    property int cellHeight: 96
    property int cellWidth: 240
    default property alias sample: holder.data

    width: root.cellWidth
    height: root.cellHeight
    Accessible.role: Accessible.Grouping
    Accessible.name: root.caption

    Rectangle {
        anchors.fill: parent
        radius: UI.Tokens.rXs
        color: UI.Tokens.bgPanel
        border.width: 1
        border.color: UI.Tokens.borderSubtle
    }
    Text {
        id: captionText
        anchors.left: parent.left
        anchors.top: parent.top
        anchors.margins: UI.Tokens.sp2
        text: root.caption
        color: UI.Tokens.textMuted
        font.family: UI.Tokens.fontMono
        font.pixelSize: UI.Tokens.fsXs
    }
    Item {
        id: holder
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: captionText.bottom
        anchors.bottom: parent.bottom
        anchors.margins: UI.Tokens.sp2
    }
}
