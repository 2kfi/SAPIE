import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    property string viewState: "empty"
    signal navigateTo(string name)
    signal showMessage(string message)
    signal retryRequested()

    C.PageHeader {
        id: header
        width: parent.width
        title: "Game"
        subtitle: "Details for the selected record"
    }

    C.EmptyState {
        id: empty
        visible: root.viewState === "empty"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "No game selected"
        message: "Choose a game from your library to see its details."
        C.Button {
            text: "Back to library"
            width: 140
            onClicked: root.navigateTo("library")
        }
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
            text: "Reading game details..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton {
            width: 420
            height: 132
            lines: 5
        }
    }

    C.ErrorState {
        id: gameError
        visible: root.viewState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 112
        title: "Game files not found"
        message: "The recorded path is missing. Restore the game or remove it from the library."
        showRetry: true
        onRetried: root.retryRequested()
    }

    Item {
        id: data
        visible: root.viewState === "data"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.right: parent.right
        anchors.rightMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp4
        height: detail.implicitHeight
        Row {
            id: detail
            spacing: UI.Tokens.sp6
            C.MonogramTile {
                width: 148
                height: 222
                appid: 0
                label: "[placeholder cover]"
            }
            Column {
                spacing: UI.Tokens.sp3
                Text {
                    text: "[PLACEHOLDER game]"
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsXl
                    font.weight: Font.DemiBold
                }
                C.StatusPill { text: "Not installed"; status: "offline" }
                C.Section {
                    width: 380
                    title: "Record"
                    C.KeyValue { width: parent.width; keyName: "App ID"; value: "[none]" }
                    C.KeyValue { width: parent.width; keyName: "Path"; value: "[not set]" }
                    C.KeyValue { width: parent.width; keyName: "Interfaces"; value: "[not scanned]" }
                    C.KeyValue { width: parent.width; keyName: "Backup"; value: "[not checked]" }
                }
                Text {
                    text: "Prototype control. No game process is started."
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsXs
                }
                C.PlayButton {
                    gameName: "[PLACEHOLDER game]"
                    onClicked: root.showMessage("Prototype: no process was started.")
                }
                C.Button {
                    text: "Back to library"
                    variant: "quiet"
                    width: 140
                    onClicked: root.navigateTo("library")
                }
            }
        }
    }
}
