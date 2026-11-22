import QtQuick 2.15
import ".." as UI

Window {
    id: window
    visible: true
    width: 1280
    height: 800
    minimumWidth: 860
    minimumHeight: 560
    color: UI.Tokens.bgApp
    title: "SAPIE identity prototype (placeholder data)"

    PrototypeApp {
        anchors.fill: parent
    }
}
