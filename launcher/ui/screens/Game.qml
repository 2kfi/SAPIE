import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "Game details"

    property var bridge: UI.FakeCoreBridge
    readonly property var record: bridge.game || ({})
    readonly property string gameId: record.id || ""
    signal navigateTo(string name)
    signal showMessage(string message)
    signal confirmRequested(string title, string message, var action)

    C.PageHeader {
        id: header
        width: parent.width
        title: record.name || "Game"
        subtitle: "Details for the selected record"
        C.Button {
            text: "Library"
            variant: "quiet"
            height: 44
            width: 100
            onClicked: root.navigateTo("library")
        }
    }

    C.EmptyState {
        id: empty
        visible: root.bridge.gameState === "empty"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "No game selected"
        message: "Choose a game from your library to see its details."
        C.Button {
            text: "Back to library"
            height: 44
            width: 140
            onClicked: root.navigateTo("library")
        }
    }

    Column {
        id: loading
        visible: root.bridge.gameState === "loading"
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
        C.LoadingSkeleton { width: 420; height: 132; lines: 5 }
    }

    C.ErrorState {
        id: error
        visible: root.bridge.gameState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 116
        title: "Game details unreadable"
        message: "The record could not be loaded. Reopen it from your library."
        showRetry: true
        onRetried: root.bridge.getGame(root.gameId)
    }

    Flickable {
        id: scroller
        visible: root.bridge.gameState === "ready" || root.bridge.gameState === "missing"
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.margins: UI.Tokens.sp4
        contentWidth: content.implicitWidth
        contentHeight: content.implicitHeight
        clip: true
        flickableDirection: Flickable.VerticalFlick

        Column {
            id: content
            width: scroller.width
            spacing: UI.Tokens.sp4

            Row {
                objectName: "gameOfflineBar"
                visible: {
                    for (var i = 0; i < root.bridge.games.length; i++)
                        if (root.bridge.games[i].id === root.gameId) return false
                    return true
                }
                width: parent.width
                spacing: UI.Tokens.sp2
                C.OfflineBanner {
                    objectName: "gameOfflineBanner"
                    width: parent.width - 128
                    message: "Server offline. Saved game settings are still available."
                }
                C.Button {
                    objectName: "gameOfflineRetry"
                    text: "Retry"
                    variant: "quiet"
                    height: 44
                    width: 112
                    onClicked: root.bridge.listGames()
                }
            }

            Row {
                id: summary
                spacing: UI.Tokens.sp5
                width: parent.width

                C.GameArt {
                    id: cover
                    width: 148
                    source: root.record.cover || ""
                    label: root.record.name || "[PLACEHOLDER game]"
                    appid: root.record.appid || 0
                }

                Column {
                    width: summary.width - cover.width - summary.spacing
                    spacing: UI.Tokens.sp3

                    Text {
                        text: root.record.name || "[PLACEHOLDER game]"
                        color: UI.Tokens.textPrimary
                        font.family: UI.Tokens.fontUi
                        font.pixelSize: UI.Tokens.fsXl
                        font.weight: Font.DemiBold
                        Accessible.role: Accessible.Heading
                        Accessible.name: text
                    }

                    Row {
                        spacing: UI.Tokens.sp3
                        C.StatusPill {
                            text: root.bridge.gameState === "missing" ? "Game file missing"
                                  : (root.record.installed ? "Library installed" : "Library not installed")
                            status: root.bridge.gameState === "missing" ? "error"
                                    : (root.record.installed ? "success" : "offline")
                        }
                        C.StatusPill {
                            visible: root.record.favorite === true
                            text: "Favorite"
                            status: "info"
                        }
                    }

                    C.PlayButton {
                        id: play
                        objectName: "gamePlayButton"
                        height: 44
                        width: 120
                        gameName: root.record.name || "game"
                        launching: root.bridge.launchState === "launching"
                        enabled: root.bridge.gameState === "ready" && root.bridge.launchState !== "launching"
                        onClicked: {
                            root.bridge.playGame(root.gameId)
                            root.showMessage("Launch requested for " + root.record.name + ".")
                        }
                    }

                    C.ErrorState {
                        id: launchError
                        objectName: "launchError"
                        visible: root.bridge.launchState === "failed"
                        width: Math.min(420, parent.width)
                        height: 116
                        title: "Launch failed"
                        message: root.bridge.launchError || "The game process did not start. Check the log for details."
                        showRetry: true
                        onRetried: root.bridge.playGame(root.gameId)
                    }

                    C.Button {
                        objectName: "favoriteButton"
                        text: root.record.favorite ? "Remove from favorites" : "Add to favorites"
                        variant: "quiet"
                        height: 44
                        width: 188
                        onClicked: {
                            root.bridge.setFavorite(root.gameId, !root.record.favorite)
                            root.showMessage("Favorites updated for " + root.record.name + ".")
                        }
                    }
                }
            }

            C.ErrorState {
                id: missingError
                objectName: "missingGameError"
                visible: root.bridge.gameState === "missing"
                width: parent.width
                height: 116
                title: "Game file missing"
                message: "The recorded path is missing. Locate the game, or remove its record from the library."
                showRetry: true
                onRetried: root.bridge.getGame(root.gameId)
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Record"
                C.KeyValue { width: parent.width; keyName: "Path"; value: root.record.path || "[not set]" }
                C.KeyValue { width: parent.width; keyName: "App ID"; value: String(root.record.appid || 0) }
                C.KeyValue { width: parent.width; keyName: "Architecture"; value: root.record.arch || "[not scanned]" }
                C.KeyValue { width: parent.width; keyName: "Era"; value: root.record.era || "[not scanned]" }
                C.KeyValue { width: parent.width; keyName: "Interfaces"; value: root.record.interfaces === undefined ? "[not scanned]" : String(root.record.interfaces) }
                C.KeyValue { width: parent.width; keyName: "Original backup"; value: root.record.backup || "[not checked]" }
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Launch options"
                C.Input {
                    id: argsField
                    objectName: "launchArgs"
                    width: parent.width
                    label: "Launch arguments"
                    text: root.record.arguments || ""
                }
                C.Input {
                    id: preloadField
                    objectName: "preloadOverride"
                    width: parent.width
                    label: "Linux preload override"
                    text: root.record.preload || ""
                }
                C.Button {
                    objectName: "saveOptionsButton"
                    text: "Save launch options"
                    height: 44
                    width: 180
                    onClicked: {
                        root.bridge.saveGameOptions(root.gameId, argsField.text, preloadField.text)
                        root.showMessage("Launch options saved for " + root.record.name + ".")
                    }
                }
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Achievements"
                Column {
                    width: parent.width
                    spacing: UI.Tokens.sp2
                    Repeater {
                        model: root.record.achievements || []
                        delegate: Item {
                            id: row
                            required property var modelData
                            width: parent.width
                            height: 24
                            Accessible.role: Accessible.ListItem
                            Accessible.name: row.modelData.hidden ? "Hidden achievement" : row.modelData.name
                            Text {
                                anchors.fill: parent
                                verticalAlignment: Text.AlignVCenter
                                text: row.modelData.hidden ? "Hidden achievement"
                                      : (row.modelData.unlocked ? row.modelData.name : row.modelData.name + " (locked)")
                                color: UI.Tokens.textSecondary
                                font.family: UI.Tokens.fontUi
                                font.pixelSize: UI.Tokens.fsSm
                            }
                        }
                    }
                    Text {
                        visible: (root.record.achievements || []).length === 0
                        width: parent.width
                        text: "No achievement data from your server."
                        color: UI.Tokens.textSecondary
                        font.family: UI.Tokens.fontUi
                        font.pixelSize: UI.Tokens.fsSm
                        Accessible.role: Accessible.StaticText
                        Accessible.name: text
                    }
                }
            }

            C.Section {
                width: Math.min(460, parent.width)
                title: "Recent launches"
                Column {
                    width: parent.width
                    spacing: UI.Tokens.sp2
                    Repeater {
                        model: root.record.recentLaunches || []
                        delegate: Item {
                            id: launchRow
                            required property var modelData
                            width: parent.width
                            height: 24
                            Accessible.role: Accessible.ListItem
                            Accessible.name: launchRow.modelData.label || "Recent launch"
                            Text {
                                anchors.fill: parent
                                verticalAlignment: Text.AlignVCenter
                                text: launchRow.modelData.label || "[TEST launch record]"
                                color: UI.Tokens.textSecondary
                                font.family: UI.Tokens.fontMono
                                font.pixelSize: UI.Tokens.fsXs
                            }
                        }
                    }
                    Text {
                        visible: (root.record.recentLaunches || []).length === 0
                        width: parent.width
                        text: "No launches recorded yet."
                        color: UI.Tokens.textSecondary
                        font.family: UI.Tokens.fontUi
                        font.pixelSize: UI.Tokens.fsSm
                        Accessible.role: Accessible.StaticText
                        Accessible.name: text
                    }
                }
            }

            Row {
                spacing: UI.Tokens.sp2
                C.Button {
                    objectName: "repairButton"
                    text: "Repair library"
                    variant: "quiet"
                    height: 44
                    width: 132
                    onClicked: root.confirmRequested("Repair library",
                        "Reinstall the matching Steam library for " + root.record.name + "?",
                        function () { root.bridge.repairGame(root.gameId) })
                }
                C.Button {
                    objectName: "restoreButton"
                    text: "Restore original"
                    variant: "danger"
                    height: 44
                    width: 148
                    onClicked: root.confirmRequested("Restore original library",
                        "Put the original library back and remove SAPIE's copy for " + root.record.name + "?",
                        function () { root.bridge.restoreGame(root.gameId) })
                }
                C.Button {
                    objectName: "removeGameButton"
                    text: "Remove from library"
                    variant: "danger"
                    height: 44
                    width: 168
                    onClicked: root.confirmRequested("Remove from library",
                        "Remove " + root.record.name + " from your library? The game files stay on disk.",
                        function () {
                            root.bridge.removeGame(root.gameId)
                            root.navigateTo("library")
                        })
                }
            }
        }
    }
}
