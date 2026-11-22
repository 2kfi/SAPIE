import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string title: ""
    property string message: ""
    default property alias action: footer.data

    implicitHeight: column.implicitHeight
    Accessible.role: Accessible.StaticText
    Accessible.name: root.title + ". " + root.message

    Column {
        id: column
        anchors.left: parent.left
        anchors.right: parent.right
        spacing: UI.Tokens.sp2
        Rectangle {
            width: 24
            height: 24
            radius: UI.Tokens.rXs
            color: "transparent"
            border.width: 1
            border.color: UI.Tokens.borderSubtle
            Accessible.ignored: true
        }
        Text {
            text: root.title
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        Text {
            text: root.message
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
            wrapMode: Text.Wrap
        }
        Item {
            id: footer
            width: parent.width
            height: childrenRect.height
        }
    }
}
