import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    property string viewState: "empty"
    signal navigateTo(string name)
    signal showMessage(string message)
    signal retryRequested()
    signal setReducedMotion(bool value)

    C.PageHeader {
        id: header
        width: parent.width
        title: "Settings"
        subtitle: "Account, server and appearance"
    }

    C.EmptyState {
        id: empty
        visible: root.viewState === "empty"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "No server configured"
        message: "Sign in to store a server address and a session token."
        C.Button {
            text: "Open sign in"
            width: 130
            onClicked: root.navigateTo("signin")
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
            text: "Reading settings..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton {
            width: 380
            height: 108
            lines: 4
        }
    }

    C.ErrorState {
        id: settingsError
        visible: root.viewState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 112
        title: "Settings file unreadable"
        message: "The local settings file could not be read. Check permissions and try again."
        showRetry: true
        onRetried: root.retryRequested()
    }

    Flickable {
        id: data
        visible: root.viewState === "data"
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.margins: UI.Tokens.sp4
        contentWidth: settingsColumn.implicitWidth
        contentHeight: settingsColumn.implicitHeight
        clip: true
        Column {
            id: settingsColumn
            spacing: UI.Tokens.sp5
            C.Section {
                width: 420
                title: "Account"
                C.KeyValue { width: parent.width; keyName: "Server"; value: "[not set]" }
                C.KeyValue { width: parent.width; keyName: "Account"; value: "[not signed in]" }
                C.StatusPill { text: "Offline"; status: "offline" }
            }
            C.Section {
                width: 420
                title: "Appearance"
                C.Checkbox {
                    id: motionToggle
                    objectName: "reducedMotionToggle"
                    label: "Reduced motion"
                    checked: false
                    onToggled: root.setReducedMotion(checked)
                }
                Text {
                    width: 420
                    text: "In-app fallback. Qt QML has no reduced-motion property, so the platform layer supplies it in production."
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsXs
                    wrapMode: Text.Wrap
                }
            }
            C.Section {
                width: 420
                title: "Identity, provisional"
                Image {
                    id: wordmark
                    width: 272
                    height: 64
                    fillMode: Image.PreserveAspectFit
                    source: "../../../design/wordmark.svg"
                    Accessible.role: Accessible.Graphic
                    Accessible.name: "Provisional SAPIE wordmark, candidate A"
                }
                Image {
                    width: 32
                    height: 32
                    fillMode: Image.PreserveAspectFit
                    source: "../../../design/icons/mark-directions/candidate-a-nested.svg"
                    Accessible.role: Accessible.Graphic
                    Accessible.name: "Provisional SAPIE mark, candidate A"
                }
                Text {
                    width: 420
                    text: "Mark candidate A, nested. Provisional art pending G1."
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsXs
                    wrapMode: Text.Wrap
                }
            }
            C.Divider { width: 420 }
            C.Button {
                text: "Sign out"
                variant: "quiet"
                width: 120
                onClicked: {
                    root.navigateTo("signin")
                    root.showMessage("Prototype: local token state was not changed.")
                }
            }
        }
    }
}
