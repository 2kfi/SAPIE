import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property int railWidth: 164
    property int toolbarHeight: 44
    property alias sidebar: rail.data
    property alias toolbar: bar.data
    default property alias content: contentArea.data

    Accessible.role: Accessible.Client
    Accessible.name: "SAPIE main window"

    Rectangle {
        anchors.fill: parent
        color: UI.Tokens.bgApp
    }
    Row {
        anchors.fill: parent
        Item {
            id: rail
            width: root.railWidth
            height: parent.height
        }
        Column {
            width: parent.width - root.railWidth
            height: parent.height
            Item {
                id: bar
                width: parent.width
                height: root.toolbarHeight
            }
            Item {
                id: contentArea
                width: parent.width
                height: parent.height - root.toolbarHeight
            }
        }
    }
}
