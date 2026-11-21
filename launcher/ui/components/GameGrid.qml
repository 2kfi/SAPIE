import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property int columns: 5
    default property alias items: grid.data

    Accessible.role: Accessible.List
    Accessible.name: "Games"

    Grid {
        id: grid
        anchors.top: parent.top
        anchors.left: parent.left
        columns: root.columns
        spacing: UI.Tokens.sp3
    }
    height: grid.implicitHeight
}
