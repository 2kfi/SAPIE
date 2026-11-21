import QtQuick 2.15
import "." as P
import ".." as UI

Item {
    id: root
    property string title: ""
    property string message: ""
    property bool showRetry: false
    signal retried()

    implicitHeight: 96
    Accessible.role: Accessible.AlertMessage
    Accessible.name: root.title + ". " + root.message

    Rectangle {
        anchors.fill: parent
        radius: UI.Tokens.rXs
        color: UI.Tokens.bgPanel
        border.width: 1
        border.color: UI.Tokens.danger
    }
    Column {
        anchors.fill: parent
        anchors.margins: UI.Tokens.sp3
        spacing: UI.Tokens.sp2
        Text {
            text: root.title
            color: UI.Tokens.danger
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        Text {
            width: parent.width
            text: root.message
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
            wrapMode: Text.Wrap
        }
        P.Button {
            visible: root.showRetry
            text: "Retry"
            variant: "danger"
            onClicked: root.retried()
        }
    }
}
