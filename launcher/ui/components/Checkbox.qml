import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

CheckBox {
    id: root
    property string label: ""
    property bool demoFocus: false

    implicitHeight: 32
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.CheckBox
    Accessible.name: root.label
    Accessible.checked: root.checked

    indicator: Rectangle {
        width: 16
        height: 16
        anchors.verticalCenter: parent.verticalCenter
        radius: UI.Tokens.rXs
        color: root.checked ? UI.Tokens.accentSoft : UI.Tokens.bgPanel
        border.width: 1
        border.color: root.checked ? UI.Tokens.accent : UI.Tokens.borderStrong
        Rectangle {
            anchors.centerIn: parent
            width: 8
            height: 8
            radius: UI.Tokens.rXs
            color: UI.Tokens.accent
            visible: root.checked
        }
    }
    contentItem: Text {
        text: root.label
        color: UI.Tokens.textPrimary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
        verticalAlignment: Text.AlignVCenter
        leftPadding: UI.Tokens.sp2
    }
    FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
}
