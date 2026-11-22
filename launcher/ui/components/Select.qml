pragma ComponentBehavior: Bound
import QtQuick 2.15
import QtQuick.Controls 2.15
import ".." as UI

ComboBox {
    id: root
    property string label: ""
    property bool demoFocus: false

    implicitWidth: 160
    implicitHeight: 32
    focusPolicy: Qt.StrongFocus
    Accessible.role: Accessible.ComboBox
    Accessible.name: root.label

    background: Rectangle {
        radius: UI.Tokens.rXs
        color: UI.Tokens.bgPanel
        border.width: 1
        border.color: (root.activeFocus || root.demoFocus || root.popup.visible) ? UI.Tokens.accent : UI.Tokens.borderStrong
    }
    contentItem: Text {
        text: root.displayText
        color: UI.Tokens.textPrimary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
        verticalAlignment: Text.AlignVCenter
        leftPadding: UI.Tokens.sp2
        elide: Text.ElideRight
    }
    indicator: Text {
        anchors.right: parent.right
        anchors.rightMargin: UI.Tokens.sp2
        anchors.verticalCenter: parent.verticalCenter
        text: "v"
        color: UI.Tokens.textSecondary
        font.pixelSize: UI.Tokens.fsXs
    }
    FocusRing {
        target: root
        cornerRadius: UI.Tokens.rXs
        visible: root.activeFocus || root.demoFocus
    }
    popup: Popup {
        y: root.height + 2
        width: root.width
        implicitHeight: Math.min(list.contentHeight, 200)
        padding: 0
        background: Rectangle {
            radius: UI.Tokens.rSm
            color: UI.Tokens.bgElevated
            border.width: 1
            border.color: UI.Tokens.borderStrong
        }
        contentItem: ListView {
            id: list
            model: root.model
            implicitHeight: contentHeight
            clip: true
            delegate: ItemDelegate {
                id: item
                required property string modelData
                required property int index
                width: ListView.view.width
                text: item.modelData
                contentItem: Text {
                    text: item.modelData
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsSm
                    verticalAlignment: Text.AlignVCenter
                }
                background: Rectangle {
                    color: item.highlighted ? UI.Tokens.bgPanelHover : "transparent"
                }
                Accessible.role: Accessible.MenuItem
                Accessible.name: item.modelData
                onClicked: {
                    root.currentIndex = item.index
                    root.popup.close()
                }
            }
        }
    }
}
