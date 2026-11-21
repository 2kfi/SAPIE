import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "About"

    property var bridge: UI.FakeCoreBridge
    signal navigateTo(string name)
    signal showMessage(string message)

    C.PageHeader {
        id: header
        width: parent.width
        title: "About"
        subtitle: "SAPIE launcher"
    }

    Column {
        id: loading
        visible: root.bridge.settingsState === "loading"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        spacing: UI.Tokens.sp3
        Text {
            text: "Reading build details..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton { width: 380; height: 108; lines: 4 }
    }

    C.ErrorState {
        id: error
        objectName: "aboutError"
        visible: root.bridge.settingsState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 116
        title: "Build details unavailable"
        message: "The launcher could not read its own build record. Retry or restart the launcher."
        showRetry: true
        onRetried: root.bridge.getPreference("version", "")
    }

    Flickable {
        id: scroller
        visible: root.bridge.settingsState === "ready"
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.margins: UI.Tokens.sp4
        contentWidth: body.implicitWidth
        contentHeight: body.implicitHeight
        clip: true
        flickableDirection: Flickable.VerticalFlick

        Column {
            id: body
            width: scroller.width
            spacing: UI.Tokens.sp5

            C.Section {
                width: Math.min(460, parent.width)
                title: "Build"
                C.KeyValue { width: parent.width; keyName: "Version"; value: root.bridge.version }
                C.KeyValue { width: parent.width; keyName: "Update channel"; value: root.bridge.updateChannel }
                C.KeyValue { width: parent.width; keyName: "Log folder"; value: root.bridge.dataFolder }
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Licenses"
                Text {
                    width: parent.width
                    text: "The Goldberg fork is LGPLv3. Modifications stay published under the same license. Qt is used through dynamic linking under LGPLv3. Steamless is not modified."
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsSm
                    wrapMode: Text.Wrap
                    Accessible.role: Accessible.StaticText
                    Accessible.name: text
                }
            }

            Row {
                spacing: UI.Tokens.sp2
                C.Button {
                    objectName: "copyDiagnosticsButton"
                    text: "Copy diagnostics"
                    height: 44
                    width: 168
                    onClicked: {
                        root.bridge.copyDiagnostics()
                        root.showMessage("Diagnostics copied.")
                    }
                }
                C.Button {
                    text: "Open data folder"
                    variant: "quiet"
                    height: 44
                    width: 164
                    onClicked: {
                        root.bridge.openDataFolder()
                        root.showMessage("Data folder requested.")
                    }
                }
                C.Button {
                    text: "Logs"
                    variant: "quiet"
                    height: 44
                    width: 100
                    onClicked: root.navigateTo("logs")
                }
            }
        }
    }
}
