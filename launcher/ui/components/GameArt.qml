import QtQuick 2.15
import ".." as UI

Item {
    id: root
    property url source: ""
    property string label: ""
    property int appid: 0

    height: width * 1.5
    Accessible.role: Accessible.Graphic
    Accessible.name: root.label + ", cover"

    MonogramTile {
        id: fallback
        anchors.fill: parent
        appid: root.appid
        label: root.label
        visible: root.source.toString() === ""
    }
    Image {
        anchors.fill: parent
        source: root.source
        fillMode: Image.PreserveAspectCrop
        visible: root.source.toString() !== ""
        Accessible.ignored: true
    }
    Rectangle {
        anchors.fill: parent
        radius: UI.Tokens.rXs
        color: "transparent"
        border.width: 1
        border.color: UI.Tokens.borderSubtle
        Accessible.ignored: true
    }
}
