import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "Settings"

    property var bridge: UI.FakeCoreBridge
    signal navigateTo(string name)
    signal showMessage(string message)
    signal confirmRequested(string title, string message, var action)

    C.PageHeader {
        id: header
        width: parent.width
        title: "Settings"
        subtitle: "Account, server and appearance"
        C.Button {
            objectName: "saveSettingsButton"
            text: "Save settings"
            height: 44
            width: 140
            onClicked: {
                root.bridge.setUpdateChannel(channelField.currentText)
                root.bridge.setReducedMotion(motionField.checked)
                root.showMessage("Settings saved.")
            }
        }
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
            text: "Reading settings..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton { width: 380; height: 108; lines: 4 }
    }

    C.ErrorState {
        id: error
        visible: root.bridge.settingsState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 116
        title: "Settings could not be read"
        message: "The local settings file is unreadable. Check permissions and try again."
        showRetry: true
        onRetried: root.bridge.settingsState = "ready"
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

            C.OfflineBanner {
                visible: root.bridge.connectionState !== "signedIn"
                width: Math.min(460, parent.width)
                message: "Not signed in. Account actions need a server connection."
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Account"
                C.KeyValue {
                    width: parent.width
                    keyName: "Server"
                    value: root.bridge.serverAddress || "[not set]"
                }
                C.KeyValue {
                    width: parent.width
                    keyName: "Account"
                    value: root.bridge.accountName || "[not signed in]"
                }
                C.Button {
                    objectName: "signOutButton"
                    text: "Sign out"
                    variant: "quiet"
                    height: 44
                    width: 120
                    onClicked: root.confirmRequested("Sign out",
                        "Sign out and clear the local session token?",
                        function () {
                            root.bridge.signOut()
                            root.navigateTo("signin")
                        })
                }
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Updates"
                C.KeyValue { width: parent.width; keyName: "Launcher"; value: root.bridge.version }
                C.Select {
                    id: channelField
                    objectName: "updateChannel"
                    width: 160
                    label: "Update channel"
                    model: ["stable", "beta"]
                    currentIndex: root.bridge.updateChannel === "beta" ? 1 : 0
                    onActivated: root.bridge.setUpdateChannel(channelField.currentText)
                }
                C.StatusPill {
                    text: root.bridge.updateState === "failed" ? "Update check failed"
                          : root.bridge.updateState === "current" ? "Up to date"
                          : "No update check yet"
                    status: root.bridge.updateState === "failed" ? "error"
                            : root.bridge.updateState === "current" ? "success" : "info"
                }
                C.Button {
                    objectName: "checkUpdatesButton"
                    text: "Check for updates"
                    variant: "quiet"
                    height: 44
                    width: 164
                    onClicked: {
                        root.bridge.checkForUpdates()
                        root.showMessage("Update check finished.")
                    }
                }
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Appearance"
                C.Checkbox {
                    id: motionField
                    objectName: "reducedMotionToggle"
                    label: "Reduced motion"
                    checked: root.bridge.reducedMotion
                    onToggled: root.bridge.setReducedMotion(checked)
                }
                Text {
                    width: parent.width
                    text: "Qt QML has no reduced-motion property. The platform layer supplies it in production; this checkbox is the in-app fallback."
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsXs
                    wrapMode: Text.Wrap
                    Accessible.role: Accessible.StaticText
                    Accessible.name: text
                }
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Diagnostics"
                C.Button {
                    objectName: "openDataFolderButton"
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
                    objectName: "copyLogButton"
                    text: "Copy log"
                    variant: "quiet"
                    height: 44
                    width: 120
                    onClicked: {
                        root.bridge.copyLog()
                        root.showMessage("Log copied.")
                    }
                }
                C.Button {
                    objectName: "viewLogsButton"
                    text: "View logs"
                    variant: "quiet"
                    height: 44
                    width: 120
                    onClicked: root.navigateTo("logs")
                }
            }
        }
    }
}
