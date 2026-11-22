pragma ComponentBehavior: Bound
import QtQuick 2.15
import QtTest 1.2
import "." as S
import ".." as UI

Item {
    id: root
    width: 860
    height: 560
    property var app: null
    property bool captured: false

    function resetState() {
        var b = UI.FakeCoreBridge
        b.connectionState = "signedIn"
        b.firstRun = false
        b.libraryState = "ready"
        b.games = [
            { id: "test-1", name: "[PLACEHOLDER game A]", appid: 0, cover: "", installed: true,
              missing: false, favorite: false, lastPlayed: "", playtimeSeconds: -1 },
            { id: "test-2", name: "[PLACEHOLDER game B]", appid: 0, cover: "", installed: false,
              missing: false, favorite: false, lastPlayed: "", playtimeSeconds: -1 }
        ]
        b.gameState = "ready"
        b.game = b.games[0]
        b.addState = "idle"
        b.friendsState = "ready"
        b.friends = [{ id: "test-p1", name: "[PLACEHOLDER player A]", status: "online" }]
        b.requests = []
        b.blocked = []
        b.lobbies = []
        b.settingsState = "ready"
        b.logsState = "ready"
        b.logs = []
        b.launchState = "idle"
        b.launchError = ""
        b.reducedMotion = false
        b.prefs = { librarySearch: "", libraryFilter: "All games", librarySort: "Name" }
        if (root.app) root.app.currentScreen = "library"
    }

    TestCase {
        name: "LauncherScreens"
        when: windowShown

        function initTestCase() {
            var component = Qt.createComponent(Qt.resolvedUrl("LauncherApp.qml"))
            compare(component.status, Component.Ready, "launcher loads")
            root.app = component.createObject(root, { width: 860, height: 560 })
            verify(root.app !== null, "launcher object exists")
            root.resetState()
            wait(100)
        }

        function cleanupTestCase() {
            if (root.app) root.app.destroy()
        }

        function init() {
            root.resetState()
        }

        function test_navigation_reaches_every_screen() {
            var routes = [
                ["navLibrary", "libraryScreen"],
                ["navAddGame", "addGameScreen"],
                ["navFriends", "friendsScreen"],
                ["navRequests", "friendsScreen"],
                ["navBlocked", "friendsScreen"],
                ["navLobbies", "friendsScreen"],
                ["navSettings", "settingsScreen"],
                ["navLogs", "logsScreen"],
                ["navAbout", "aboutScreen"]
            ]
            for (var i = 0; i < routes.length; i++) {
                findChild(root.app, routes[i][0]).clicked()
                compare(findChild(root.app, routes[i][1]).visible, true, routes[i][1] + " visible")
            }
        }

        function test_library_empty_and_large_grid() {
            UI.FakeCoreBridge.games = []
            root.app.currentScreen = "library"
            compare(findChild(root.app, "emptyAddGameButton").visible, true, "empty state action visible")

            var many = []
            for (var i = 0; i < 100; i++) {
                many.push({ id: "test-" + i, name: "[PLACEHOLDER game " + i + "]", appid: i,
                    cover: "", installed: i % 2 === 0, missing: false, favorite: false,
                    lastPlayed: "", playtimeSeconds: -1 })
            }
            UI.FakeCoreBridge.games = many
            var grid = findChild(root.app, "gameGrid")
            compare(grid.visible, true, "grid visible with 100 records")
            verify(grid.implicitWidth > 0, "grid has content")
            verify(grid.implicitWidth <= 860, "grid does not overflow the narrow window")
        }

        function test_search_filter_and_sort_persist() {
            root.app.currentScreen = "library"
            var library = findChild(root.app, "libraryScreen")
            library.searchText = "B"
            compare(library.shown.length, 1, "search filters placeholder records")
            compare(UI.FakeCoreBridge.prefs.librarySearch, "B", "search persists to bridge")
            library.searchText = ""
            library.filterName = "Installed"
            compare(library.shown.length, 1, "installed filter works")
            library.filterName = "All games"
            library.sortName = "Last played"
            compare(UI.FakeCoreBridge.prefs.librarySort, "Last played", "sort persists to bridge")
        }

        function test_dialog_focus_trap_and_escape() {
            root.app.currentScreen = "friends"
            var trigger = findChild(root.app, "addFriendButton")
            var dialog = findChild(root.app, "addFriendDialog")
            var cancel = findChild(dialog, "addFriendCancel")
            var confirm = findChild(dialog, "addFriendConfirm")
            trigger.clicked()
            tryCompare(dialog, "open", true)
            tryVerify(function () { return cancel.activeFocus }, 2000)
            keyPress(Qt.Key_Tab)
            tryVerify(function () { return confirm.activeFocus }, 2000)
            keyPress(Qt.Key_Tab)
            tryVerify(function () { return cancel.activeFocus }, 2000)
            keyPress(Qt.Key_Escape)
            tryCompare(dialog, "open", false)
            tryVerify(function () { return trigger.activeFocus }, 2000)
        }

        function test_destructive_action_needs_confirmation() {
            root.app.currentScreen = "game"
            var restore = findChild(root.app, "restoreButton")
            var dialog = findChild(root.app, "confirmDialog")
            restore.clicked()
            tryCompare(dialog, "open", true)
            compare(UI.FakeCoreBridge.lastAction !== "restoreGame", true, "no action before confirm")
            findChild(dialog, "confirmAccept").clicked()
            tryCompare(dialog, "open", false)
            compare(UI.FakeCoreBridge.lastAction, "restoreGame", "restore runs after confirm")
        }

        function test_duplicate_view_existing_uses_detected_record() {
            var b = UI.FakeCoreBridge
            b.addState = "duplicate"
            b.detection = { path: "test-2", arch: "x86", era: "modern", interfaces: 1,
                appid: 0, duplicate: true }
            root.app.currentScreen = "add"
            var button = findChild(root.app, "viewExistingButton")
            verify(button !== null, "view existing button exists")
            b.lastAction = ""
            button.clicked()
            compare(b.lastAction, "getGame", "duplicate action reads a library record")
            compare(b.game.id, "test-2", "the detected record opens, not a hard-coded id")
        }

        function test_launch_failed_state_has_message_and_retry() {
            root.app.currentScreen = "game"
            UI.FakeCoreBridge.launchState = "failed"
            UI.FakeCoreBridge.launchError = "The game process exited before it connected."
            var failure = findChild(root.app, "launchError")
            compare(failure.visible, true, "launch failure visible")
            compare(failure.message, "The game process exited before it connected.", "specific message")
            compare(failure.showRetry, true, "retry action present")
        }

        function test_state_messages_have_actions() {
            var b = UI.FakeCoreBridge

            b.libraryState = "offline"
            root.app.currentScreen = "library"
            compare(findChild(root.app, "libraryOfflineBar").visible, true, "library offline visible")
            b.lastAction = ""
            findChild(root.app, "libraryOfflineRetry").clicked()
            compare(b.lastAction, "listGames", "library offline retries")

            b.games = []
            b.gameState = "ready"
            b.game = { id: "test-offline", name: "[PLACEHOLDER game]", appid: 0, cover: "",
                installed: true, missing: false, favorite: false, lastPlayed: "", playtimeSeconds: -1 }
            root.app.currentScreen = "game"
            compare(findChild(root.app, "gameOfflineBar").visible, true, "game offline visible")
            b.lastAction = ""
            findChild(root.app, "gameOfflineRetry").clicked()
            compare(b.lastAction, "listGames", "game offline retries")

            var missing = { id: "test-missing", name: "[PLACEHOLDER missing game]", appid: 0, cover: "",
                installed: false, missing: true, favorite: false, lastPlayed: "", playtimeSeconds: -1 }
            b.games = [missing]
            b.game = missing
            b.gameState = "missing"
            var missingError = findChild(root.app, "missingGameError")
            compare(missingError.showRetry, true, "missing game has an action")
            b.lastAction = ""
            missingError.retried()
            compare(b.lastAction, "getGame", "missing game retries")

            b.friendsState = "offline"
            root.app.currentScreen = "friends"
            compare(findChild(root.app, "friendsOfflineBar").visible, true, "friends offline visible")
            b.lastAction = ""
            findChild(root.app, "friendsOfflineRetry").clicked()
            compare(b.lastAction, "listFriends", "friends offline retries")

            b.settingsState = "error"
            root.app.currentScreen = "about"
            compare(findChild(root.app, "aboutError").showRetry, true, "about error has an action")
        }

        function test_accessible_names_and_roles() {
            var names = ["navLibrary", "navAddGame", "navFriends", "navSettings", "libraryScreen"]
            for (var i = 0; i < names.length; i++) {
                var item = findChild(root.app, names[i])
                verify(item !== null, names[i] + " exists")
                verify(item.Accessible.name !== "", names[i] + " has an accessible name")
                verify(item.Accessible.role !== Accessible.NoRole, names[i] + " has a role")
            }
            var play = findChild(root.app, "gamePlayButton")
            compare(play.Accessible.name, "Play " + UI.FakeCoreBridge.games[0].name, "play action named")
        }

        function test_narrow_window_grab() {
            compare(root.app.width, 860, "narrow width")
            compare(root.app.height, 560, "narrow height")
            root.captured = false
            var stage = findChild(root.app, "gameGrid")
            stage.grabToImage(function (result) {
                root.captured = result !== null
            })
            tryVerify(function () { return root.captured }, 3000)
        }

        function test_signed_out_shows_onboarding() {
            UI.FakeCoreBridge.connectionState = "signedOut"
            UI.FakeCoreBridge.firstRun = true
            compare(findChild(root.app, "signInScreen").visible, true, "sign in visible")
            compare(findChild(root.app, "setupServerButton").visible, true, "first run action visible")
            findChild(root.app, "setupServerButton").clicked()
            compare(UI.FakeCoreBridge.firstRun, false, "first run advances to form")
        }
    }
}
