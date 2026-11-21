import QtQuick 2.15
import QtQuick.Controls 2.15
import "." as P
import ".." as UI

TextField {
    id: root
    property string label: "Search"
    property bool demoFocus: false

    implicitWidth: 200
    implicitHeight: 32
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.EditableText
    Accessible.name: root.label
    placeholderText: root.label
    color: UI.Tokens.textPrimary
    placeholderTextColor: UI.Tokens.textMuted
    font.family: UI.Tokens.fontUi
    font.pixelSize: UI.Tokens.fsSm

    background: Rectangle {
        radius: UI.Tokens.rXs
        color: UI.Tokens.bgPanel
        border.width: 1
        border.color: (root.activeFocus || root.demoFocus) ? UI.Tokens.accent : UI.Tokens.borderStrong
    }
    P.FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
    P.IconButton {
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.rightMargin: UI.Tokens.sp1
        visible: root.text.length > 0
        glyph: "x"
        label: "Clear " + root.label.toLowerCase()
        onClicked: root.text = ""
    }
}
