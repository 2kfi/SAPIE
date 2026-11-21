import QtQuick 2.15
import ".." as UI
import "../components" as C
import "." as P

Item {
    id: root
    property string viewState: "empty"
    signal navigateTo(string name)
    signal showMessage(string message)
    signal retryRequested()

    C.PageHeader {
        id: header
        width: parent.width
        title: "Friends"
        subtitle: "Presence from your server"
    }

    C.EmptyState {
        id: empty
        visible: root.viewState === "empty"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "No friends added"
        message: "Add a friend by name to see presence here."
        C.Button {
            id: addFriendButton
            objectName: "addFriendButton"
            text: "Add friend"
            width: 120
            onClicked: addFriendDialog.open = true
        }
    }

    P.PrototypeDialog {
        id: addFriendDialog
        objectName: "addFriendDialog"
        title: "Add friend"
        trigger: addFriendButton
        message: "This prototype has no server connection and adds no account. Use a real launcher build to add a friend."
        onAcknowledged: root.showMessage("Prototype: no friend was added.")
    }

    Column {
        id: loading
        visible: root.viewState === "loading"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        spacing: UI.Tokens.sp3
        Text {
            text: "Loading presence..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton {
            width: 360
            height: 132
            lines: 5
        }
    }

    C.ErrorState {
        id: friendsError
        visible: root.viewState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 112
        title: "Presence is unavailable"
        message: "Your server did not answer. Cached presence is not available."
        showRetry: true
        onRetried: root.retryRequested()
    }

    Column {
        id: data
        visible: root.viewState === "data"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.right: parent.right
        anchors.rightMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp4
        spacing: UI.Tokens.sp4
        Text {
            text: "Placeholder players below. No accounts are connected."
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
        }
        C.FriendGroup {
            width: Math.min(420, parent.width)
            title: "Online"
            C.FriendRow {
                name: "[PLACEHOLDER player]"
                status: "online"
                onClicked: root.showMessage("Prototype: no account is connected.")
            }
        }
        C.FriendGroup {
            width: Math.min(420, parent.width)
            title: "Offline"
            C.FriendRow {
                name: "[PLACEHOLDER player]"
                status: "offline"
                onClicked: root.showMessage("Prototype: no account is connected.")
            }
        }
    }
}
