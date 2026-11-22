import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

TextField {
    id: root
    property string label: ""
    property bool password: false
    property bool demoFocus: false

    implicitWidth: 240
    implicitHeight: 32
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.EditableText
    Accessible.name: root.label
    placeholderText: root.label
    echoMode: root.password ? TextInput.Password : TextInput.Normal
    color: UI.Tokens.textPrimary
    placeholderTextColor: UI.Tokens.textMuted
    font.family: UI.Tokens.fontUi
    font.pixelSize: UI.Tokens.fsSm
    selectionColor: UI.Tokens.accentSoft
    selectedTextColor: UI.Tokens.textPrimary

    background: Rectangle {
        radius: UI.Tokens.rXs
        color: UI.Tokens.bgPanel
        border.width: 1
        border.color: (root.activeFocus || root.demoFocus) ? UI.Tokens.accent : UI.Tokens.borderStrong
    }
    FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
}
