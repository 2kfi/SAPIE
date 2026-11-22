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
        title: "Library"
        subtitle: "Games you added from disk"
    }

    C.EmptyState {
        id: empty
        visible: root.viewState === "empty"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "No games added"
        message: "Add a game from disk to start a library."
        C.Button {
            id: addButton
            objectName: "addGameButton"
            text: "Add a game"
            width: 120
            onClicked: addDialog.open = true
        }
    }

    P.PrototypeDialog {
        id: addDialog
        objectName: "addGameDialog"
        title: "Add a game"
        trigger: addButton
        message: "This prototype has no file picker and installs no library. Use a real launcher build to add a game."
        onAcknowledged: root.showMessage("Prototype: no game was added.")
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
            text: "Reading your library..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton {
            width: 460
            height: 132
            lines: 5
        }
    }

    C.ErrorState {
        id: libraryError
        visible: root.viewState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 112
        title: "Library could not be read"
        message: "The local library file is unreadable. Restore it or add the game again."
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
        spacing: UI.Tokens.sp3
        C.OfflineBanner { width: Math.min(520, parent.width) }
        Text {
            text: "Placeholder record below. No game data is loaded in this prototype."
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
        }
        Item { width: 1; height: UI.Tokens.sp2 }
        Row {
            spacing: UI.Tokens.sp4
            Column {
                spacing: UI.Tokens.sp2
                C.MonogramTile {
                    width: 148
                    height: 222
                    appid: 0
                    label: "[PLACEHOLDER game]"
                }
                C.StatusPill { text: "Not installed"; status: "offline" }
                C.Button {
                    text: "View details"
                    width: 120
                    onClicked: root.navigateTo("game")
                }
            }
            Column {
                anchors.verticalCenter: parent.verticalCenter
                spacing: UI.Tokens.sp2
                Text {
                    text: "[PLACEHOLDER game]"
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsLg
                }
                Text {
                    text: "App ID [none]"
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontMono
                    font.pixelSize: UI.Tokens.fsSm
                }
            }
        }
    }
}
