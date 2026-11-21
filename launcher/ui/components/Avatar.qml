import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property string name: ""
    property url source: ""
    property int size: 36

    width: root.size
    height: root.size
    Accessible.role: Accessible.Graphic
    Accessible.name: root.name + ", avatar"

    Rectangle {
        anchors.fill: parent
        radius: UI.Tokens.rSm
        color: UI.Tokens.bgElevated
        border.width: 1
        border.color: UI.Tokens.borderStrong
    }
    Image {
        anchors.fill: parent
        source: root.source
        fillMode: Image.PreserveAspectCrop
        visible: root.source.toString() !== ""
        Accessible.ignored: true
    }
    Text {
        anchors.centerIn: parent
        visible: root.source.toString() === ""
        text: root.name.length > 0 ? root.name.charAt(0).toUpperCase() : "?"
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsLg
        Accessible.ignored: true
    }
}
