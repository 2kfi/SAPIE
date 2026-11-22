import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "Add game"

    property var bridge: UI.FakeCoreBridge
    property string appid: ""
    property string era: "Unknown"
    property string args: ""
    property string preload: ""
    property bool unpackStub: false
    readonly property var detection: bridge.detection || ({})

    // The native picker fills bridge.detection.path, then the bridge enters "detecting".
    Connections {
        target: root.bridge
        function onAddStateChanged() {
            const path = root.detection.path || ""
            if (root.bridge.addState === "detecting" && path) {
                root.bridge.detectGame(path)
            }
        }
    }

    signal navigateTo(string name)
    signal showMessage(string message)

    C.PageHeader {
        id: header
        width: parent.width
        title: "Add game"
        subtitle: "Add a game you already own, from disk"
        C.Button {
            text: "Library"
            variant: "quiet"
            height: 44
            width: 100
            onClicked: root.navigateTo("library")
        }
    }

    Flickable {
        id: scroller
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
            spacing: UI.Tokens.sp4

            C.EmptyState {
                visible: root.bridge.addState === "idle"
                width: Math.min(460, parent.width)
                title: "Choose an executable"
                message: "Select the game's .exe or binary. SAPIE reads it before writing anything."
                C.Button {
                    objectName: "pickExecutableButton"
                    text: "Choose executable"
                    height: 44
                    width: 164
                    onClicked: root.bridge.chooseExecutable()
                }
            }

            Column {
                visible: root.bridge.addState === "detecting"
                spacing: UI.Tokens.sp3
                Text {
                    text: "Reading the executable..."
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsMd
                }
                C.LoadingSkeleton { width: Math.min(420, body.width); height: 108; lines: 4 }
                C.Button { text: "Reading"; height: 44; width: 120; loading: true }
            }

            C.ErrorState {
                id: detectError
                objectName: "addGameError"
                visible: root.bridge.addState === "error"
                width: Math.min(460, body.width)
                height: 140
                title: "Game could not be added"
                message: "The library file was not copied. Roll back to put the original file back, then try again."
                showRetry: false
            }

            C.EmptyState {
                visible: root.bridge.addState === "duplicate"
                width: Math.min(460, body.width)
                title: "Already in your library"
                message: "This executable is recorded. Open that record instead of adding it twice."
                C.Button {
                    objectName: "viewExistingButton"
                    text: "View existing record"
                    height: 44
                    width: 176
                    onClicked: {
                        root.bridge.getGame(root.detection.path)
                        root.navigateTo("game")
                    }
                }
            }

            Column {
                visible: root.bridge.addState === "installing"
                spacing: UI.Tokens.sp3
                Text {
                    text: "Adding " + (root.detection.path || "the selected executable") + "..."
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsMd
                }
                C.LoadingSkeleton { width: Math.min(420, body.width); height: 108; lines: 4 }
                Text {
                    text: "Copying the matching library and writing connection files."
                    color: UI.Tokens.textSecondary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsSm
                }
                C.Button {
                    objectName: "cancelInstallButton"
                    text: "Cancel"
                    variant: "quiet"
                    height: 44
                    width: 112
                    onClicked: root.bridge.cancelInstall()
                }
            }

            C.EmptyState {
                visible: root.bridge.addState === "done"
                width: Math.min(460, body.width)
                title: "Game added"
                message: "The library file is installed and the connection files are written."
                C.Button {
                    objectName: "openAddedGameButton"
                    text: "Open game details"
                    height: 44
                    width: 168
                    onClicked: root.navigateTo("game")
                }
            }

            Column {
                id: review
                visible: root.bridge.addState === "review"
                width: Math.min(520, parent.width)
                spacing: UI.Tokens.sp4

                C.Section {
                    width: parent.width
                    title: "Detected from the executable"
                    C.KeyValue { width: parent.width; keyName: "Path"; value: root.detection.path || "[not picked]" }
                    C.KeyValue { width: parent.width; keyName: "Architecture"; value: root.detection.arch || "[not scanned]" }
                    C.KeyValue { width: parent.width; keyName: "Interfaces"; value: String(root.detection.interfaces || 0) }
                }

                C.Section {
                    width: parent.width
                    title: "Review before adding"
                    C.Input {
                        id: appidField
                        objectName: "appidField"
                        width: parent.width
                        label: "App ID"
                        text: root.appid
                        onTextChanged: root.appid = text
                    }
                    Text {
                        width: parent.width
                        text: "Only a known executable gets a prefilled App ID. Change it if it is wrong."
                        color: UI.Tokens.textSecondary
                        font.family: UI.Tokens.fontUi
                        font.pixelSize: UI.Tokens.fsXs
                        wrapMode: Text.Wrap
                        Accessible.role: Accessible.StaticText
                        Accessible.name: text
                    }
                    C.Select {
                        id: eraField
                        objectName: "eraField"
                        width: 180
                        label: "Era"
                        model: ["Unknown", "Legacy", "Modern"]
                        currentIndex: 0
                        onCurrentIndexChanged: root.era = eraField.currentText
                    }
                }

                C.Checkbox {
                    id: advancedToggle
                    objectName: "advancedToggle"
                    label: "Show advanced options"
                    checked: false
                }

                C.Section {
                    visible: advancedToggle.checked
                    width: parent.width
                    title: "Advanced options"
                    C.Input {
                        id: argsField
                        objectName: "addLaunchArgs"
                        width: parent.width
                        label: "Launch arguments"
                        text: root.args
                        onTextChanged: root.args = text
                    }
                    C.Input {
                        id: preloadField
                        objectName: "addPreload"
                        width: parent.width
                        label: "Linux preload override"
                        text: root.preload
                        onTextChanged: root.preload = text
                    }
                    C.Checkbox {
                        id: stubToggle
                        label: "Unpack SteamStub (Windows)"
                        checked: root.unpackStub
                        onToggled: root.unpackStub = checked
                    }
                }

                Row {
                    spacing: UI.Tokens.sp2
                    C.Button {
                        objectName: "confirmAddButton"
                        text: "Add to library"
                        height: 44
                        width: 148
                        onClicked: {
                            if (root.appid === "") {
                                root.showMessage("Enter an App ID before adding the game.")
                                appidField.forceActiveFocus()
                                return
                            }
                            root.bridge.installGame({ appid: root.appid, era: root.era,
                                args: root.args, preload: root.preload, unpackStub: root.unpackStub,
                                duplicate: false })
                        }
                    }
                    C.Button {
                        text: "Cancel"
                        variant: "quiet"
                        height: 44
                        width: 112
                        onClicked: root.bridge.cancelInstall()
                    }
                }
            }

            Row {
                visible: root.bridge.addState === "error"
                spacing: UI.Tokens.sp2
                C.Button {
                    objectName: "rollbackButton"
                    text: "Roll back changes"
                    variant: "danger"
                    height: 44
                    width: 164
                    onClicked: {
                        root.bridge.rollbackInstall()
                        root.showMessage("Original library restored after rollback.")
                    }
                }
                C.Button {
                    text: "Try again"
                    variant: "quiet"
                    height: 44
                    width: 120
                    onClicked: root.bridge.detectGame(root.detection.path || "")
                }
            }
        }
    }
}
