pragma ComponentBehavior: Bound
import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "Library"

    property var bridge: UI.FakeCoreBridge
    property string searchText: ""
    property string filterName: "All games"
    property string sortName: "Name"
    property int cardWidth: 148
    property var filters: ["All games", "Installed", "Not installed", "Favorites", "Recent"]
    property var sorts: ["Name", "Last played"]

    signal navigateTo(string name)
    signal showMessage(string message)

    function computeShown() {
        var out = []
        var needle = root.searchText.toLowerCase()
        for (var i = 0; i < root.bridge.games.length; i++) {
            var game = root.bridge.games[i]
            if (needle !== "" && game.name.toLowerCase().indexOf(needle) < 0) continue
            if (root.filterName === "Installed" && !game.installed) continue
            if (root.filterName === "Not installed" && game.installed) continue
            if (root.filterName === "Favorites" && !game.favorite) continue
            if (root.filterName === "Recent" && !game.lastPlayed) continue
            out.push(game)
        }
        out.sort(function (a, b) {
            if (root.sortName === "Last played") {
                if (a.lastPlayed === b.lastPlayed) return a.name.localeCompare(b.name)
                return b.lastPlayed.localeCompare(a.lastPlayed)
            }
            return a.name.localeCompare(b.name)
        })
        return out
    }

    property var shown: computeShown()

    Component.onCompleted: {
        searchText = bridge.getPreference("librarySearch", "")
        filterName = bridge.getPreference("libraryFilter", "All games")
        sortName = bridge.getPreference("librarySort", "Name")
    }
    onSearchTextChanged: bridge.setPreference("librarySearch", searchText)
    onFilterNameChanged: bridge.setPreference("libraryFilter", filterName)
    onSortNameChanged: bridge.setPreference("librarySort", sortName)

    C.PageHeader {
        id: header
        width: parent.width
        title: "Library"
        subtitle: "Games you added from disk"
        C.Button {
            objectName: "addGameButton"
            text: "Add a game"
            height: 44
            width: 128
            onClicked: root.navigateTo("add")
        }
    }

    Flickable {
        id: scroller
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.leftMargin: UI.Tokens.sp4
        anchors.rightMargin: UI.Tokens.sp4
        anchors.topMargin: UI.Tokens.sp2
        contentWidth: body.implicitWidth
        contentHeight: body.implicitHeight
        clip: true
        flickableDirection: Flickable.VerticalFlick

        Column {
            id: body
            width: scroller.width
            spacing: UI.Tokens.sp3

            Row {
                id: controls
                width: parent.width
                spacing: UI.Tokens.sp2
                C.SearchField {
                    id: search
                    objectName: "librarySearch"
                    width: Math.min(200, controls.width - UI.Tokens.sp8 - 272)
                    label: "Search games"
                    text: root.searchText
                    onTextChanged: root.searchText = text
                }
                C.Select {
                    id: filterSelect
                    objectName: "libraryFilter"
                    width: 136
                    label: "Filter games"
                    model: root.filters
                    currentIndex: root.filters.indexOf(root.filterName)
                    onActivated: root.filterName = root.filters[currentIndex]
                }
                C.Select {
                    id: sortSelect
                    objectName: "librarySort"
                    width: 116
                    label: "Sort games"
                    model: root.sorts
                    currentIndex: root.sorts.indexOf(root.sortName)
                    onActivated: root.sortName = root.sorts[currentIndex]
                }
                C.Button {
                    objectName: "clearFiltersButton"
                    visible: root.searchText !== "" || root.filterName !== "All games"
                    text: "Clear filters"
                    variant: "quiet"
                    height: 44
                    width: 112
                    onClicked: {
                        root.searchText = ""
                        root.filterName = "All games"
                    }
                }
            }

            Row {
                objectName: "libraryOfflineBar"
                visible: root.bridge.libraryState === "offline"
                width: parent.width
                spacing: UI.Tokens.sp2
                C.OfflineBanner {
                    objectName: "libraryOfflineBanner"
                    width: parent.width - 128
                    message: "Server offline. Cached game records are still available."
                }
                C.Button {
                    objectName: "libraryOfflineRetry"
                    text: "Retry"
                    variant: "quiet"
                    height: 44
                    width: 112
                    onClicked: root.bridge.listGames()
                }
            }

            Column {
                id: loading
                visible: root.bridge.libraryState === "loading"
                spacing: UI.Tokens.sp3
                Text {
                    text: "Reading your library..."
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsMd
                }
                C.LoadingSkeleton { width: Math.min(560, body.width); height: 132; lines: 5 }
            }

            C.ErrorState {
                id: error
                visible: root.bridge.libraryState === "error"
                width: Math.min(460, body.width)
                height: 116
                title: "Library file could not be read"
                message: "The local library file is unreadable. Restore it or add the game again."
                showRetry: true
                onRetried: root.bridge.listGames()
            }

            C.EmptyState {
                id: noGames
                visible: root.bridge.libraryState !== "loading" && root.bridge.libraryState !== "error"
                         && root.bridge.games.length === 0
                width: Math.min(440, body.width)
                title: "No games added"
                message: "Add a game from disk to start a library."
                C.Button {
                    objectName: "emptyAddGameButton"
                    text: "Add a game"
                    height: 44
                    width: 128
                    onClicked: root.navigateTo("add")
                }
            }

            C.EmptyState {
                id: noMatches
                visible: root.bridge.libraryState === "ready" && root.bridge.games.length > 0 && root.shown.length === 0
                width: Math.min(440, body.width)
                title: "No games match"
                message: "Change the search text or clear the filters to see your library."
                C.Button {
                    text: "Clear filters"
                    variant: "quiet"
                    height: 44
                    width: 128
                    onClicked: {
                        root.searchText = ""
                        root.filterName = "All games"
                    }
                }
            }

            Grid {
                id: grid
                objectName: "gameGrid"
                visible: root.bridge.libraryState !== "loading" && root.bridge.libraryState !== "error"
                         && root.shown.length > 0
                width: parent.width
                columns: Math.max(2, Math.min(5, Math.floor((body.width + UI.Tokens.sp3) / (root.cardWidth + UI.Tokens.sp3))))
                spacing: UI.Tokens.sp3
                Repeater {
                    model: root.shown
                    delegate: Column {
                        id: cell
                        required property var modelData
                        width: root.cardWidth
                        spacing: UI.Tokens.sp2
                        Accessible.role: Accessible.Grouping
                        Accessible.name: cell.modelData.name + ", library record"

                        C.GameCard {
                            id: card
                            objectName: "gameCard"
                            width: root.cardWidth
                            height: 328
                            title: cell.modelData.name
                            appid: cell.modelData.appid
                            cover: cell.modelData.cover
                            installed: cell.modelData.installed && !cell.modelData.missing
                            selected: cell.modelData.id === root.bridge.game.id
                                      || card.activeFocus
                            onClicked: {
                                root.bridge.getGame(cell.modelData.id)
                                root.navigateTo("game")
                            }
                            onPlayRequested: {
                                root.bridge.playGame(cell.modelData.id)
                                root.showMessage("Launch requested for " + cell.modelData.name + ".")
                            }
                        }
                        Text {
                            width: parent.width
                            text: cell.modelData.playtimeSeconds >= 0
                                  ? "Playtime recorded"
                                  : "No playtime recorded"
                            color: UI.Tokens.textSecondary
                            font.family: UI.Tokens.fontUi
                            font.pixelSize: UI.Tokens.fsXs
                            elide: Text.ElideRight
                            Accessible.role: Accessible.StaticText
                            Accessible.name: text
                        }
                        Text {
                            visible: cell.modelData.favorite
                            width: parent.width
                            text: "Favorite"
                            color: UI.Tokens.accent
                            font.family: UI.Tokens.fontUi
                            font.pixelSize: UI.Tokens.fsXs
                            Accessible.role: Accessible.StaticText
                            Accessible.name: text
                        }
                        Text {
                            visible: cell.modelData.missing
                            width: parent.width
                            text: "Game file missing"
                            color: UI.Tokens.danger
                            font.family: UI.Tokens.fontUi
                            font.pixelSize: UI.Tokens.fsXs
                            Accessible.role: Accessible.StaticText
                            Accessible.name: text
                        }
                    }
                }
            }
        }
    }
}
