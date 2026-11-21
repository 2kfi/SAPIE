import QtQuick 2.15
import ".." as UI
import "../components" as C

C.Dialog {
    id: root
    title: "Prototype boundary"
    property string message: ""
    focusTargets: [cancel, understood]
    signal acknowledged()

    Text {
        width: parent.width
        text: root.message
        color: UI.Tokens.textSecondary
        font.family: UI.Tokens.fontUi
        font.pixelSize: UI.Tokens.fsSm
        wrapMode: Text.Wrap
    }
    Row {
        anchors.right: parent.right
        spacing: UI.Tokens.sp2
        C.Button {
            id: cancel
            objectName: "dialogCancel"
            text: "Cancel"
            variant: "quiet"
            onClicked: root.close()
        }
        C.Button {
            id: understood
            objectName: "dialogConfirm"
            text: "Understood"
            onClicked: {
                root.close()
                root.acknowledged()
            }
        }
    }
}
