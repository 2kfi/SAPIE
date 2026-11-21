pragma ComponentBehavior: Bound
import QtQuick 2.15
import QtTest 1.2
import ".." as UI

Item {
    id: root
    width: 860
    height: 560
    property var bridge: UI.FakeCoreBridge
    property string lastSignal: ""

    Connections {
        target: UI.FakeCoreBridge
        function onOperationFinished(operation, ok, message) {
            root.lastSignal = message
        }
    }

    TestCase {
        name: "FakeCoreBridgeInterface"
        when: windowShown

        function init() {
            var b = root.bridge
            b.connectionState = "signedOut"
            b.firstRun = true
            b.games = [{ id: "test-1", name: "[PLACEHOLDER game A]", appid: 0, cover: "",
                installed: true, missing: false, favorite: false, lastPlayed: "", playtimeSeconds: -1 }]
            b.game = b.games[0]
            b.gameState = "ready"
            b.addState = "idle"
            b.friends = [{ id: "test-p1", name: "[PLACEHOLDER player A]", status: "online" }]
            b.requests = [{ id: "test-r1", name: "[PLACEHOLDER player B]", direction: "incoming" }]
            b.blocked = [{ id: "test-b1", name: "[PLACEHOLDER player C]" }]
            b.lobbies = [{ id: "test-l1", name: "[PLACEHOLDER lobby]", members: 1, capacity: 4,
                lobbyState: "joinable" }]
            b.logs = [{ level: "Info", message: "[TEST log entry]" }]
            b.prefs = {}
            b.copiedText = ""
            b.reducedMotion = false
        }

        function test_sign_in_and_sign_out() {
            root.bridge.signIn("local:8080", "tester", "not-a-real-password", true)
            compare(root.bridge.connectionState, "signedIn", "login succeeds")
            compare(root.bridge.accountName, "tester", "account recorded")
            compare(root.bridge.rememberSession, true, "remember token only")
            root.bridge.signOut()
            compare(root.bridge.connectionState, "signedOut", "logout clears session")
        }

        function test_sign_in_validation() {
            root.bridge.signIn("", "tester", "not-a-real-password", false)
            compare(root.bridge.connectionState, "badHost", "bad host reported")
        }

        function test_library_and_game_operations() {
            root.bridge.listGames()
            compare(root.bridge.libraryState, "ready", "library ready")
            root.bridge.getGame("test-1")
            compare(root.bridge.gameState, "ready", "game ready")
            root.bridge.setFavorite("test-1", true)
            compare(root.bridge.lastAction, "setFavorite", "favorite saved")
            root.bridge.playGame("test-1")
            compare(root.bridge.launchState, "running", "launch reported")
            root.bridge.repairGame("test-1")
            compare(root.bridge.lastAction, "repairGame", "repair reported")
            root.bridge.restoreGame("test-1")
            compare(root.bridge.lastAction, "restoreGame", "restore reported")
            root.bridge.saveGameOptions("test-1", "-windowed", "")
            compare(root.bridge.lastAction, "saveGameOptions", "options saved")
            root.bridge.removeGame("test-1")
            compare(root.bridge.lastAction, "removeGame", "remove reported")
        }

        function test_add_game_review_install_and_rollback() {
            root.bridge.chooseExecutable()
            compare(root.bridge.addState, "detecting", "detection started")
            root.bridge.detectGame("[TEST path]")
            compare(root.bridge.addState, "review", "review reached")
            root.bridge.installGame({ duplicate: false })
            compare(root.bridge.addState, "done", "install completed")
            root.bridge.installGame({ duplicate: true })
            compare(root.bridge.addState, "duplicate", "duplicate detected")
            root.bridge.addState = "error"
            root.bridge.rollbackInstall()
            compare(root.bridge.addState, "idle", "rollback restores idle step")
        }

        function test_social_operations() {
            root.bridge.listFriends()
            compare(root.bridge.lastAction, "listFriends", "friends listed")
            root.bridge.addFriend("[PLACEHOLDER player D]")
            compare(root.bridge.lastAction, "addFriend", "friend request sent")
            root.bridge.acceptRequest("test-r1")
            compare(root.bridge.lastAction, "acceptRequest", "request accepted")
            root.bridge.declineRequest("test-r1")
            compare(root.bridge.lastAction, "declineRequest", "request declined")
            root.bridge.cancelRequest("test-r1")
            compare(root.bridge.lastAction, "cancelRequest", "outgoing request cancelled")
            root.bridge.blockFriend("test-p1")
            compare(root.bridge.lastAction, "blockFriend", "friend blocked")
            root.bridge.unblockFriend("test-b1")
            compare(root.bridge.lastAction, "unblockFriend", "account unblocked")
            root.bridge.removeFriend("test-p1")
            compare(root.bridge.lastAction, "removeFriend", "friend removed")
            root.bridge.listLobbies()
            compare(root.bridge.lastAction, "listLobbies", "lobbies listed")
            root.bridge.joinLobby("test-l1")
            compare(root.bridge.lastAction, "joinLobby", "lobby joined")
            root.bridge.inviteFriend("test-p1", "test-l1")
            compare(root.bridge.lastAction, "inviteFriend", "invite sent")
        }

        function test_preferences_and_diagnostics() {
            root.bridge.setPreference("librarySort", "Name")
            compare(root.bridge.getPreference("librarySort", "Other"), "Name", "preference round trip")
            compare(root.bridge.getPreference("missing", "fallback"), "fallback", "preference fallback")
            root.bridge.setUpdateChannel("beta")
            compare(root.bridge.updateChannel, "beta", "channel saved")
            root.bridge.checkForUpdates()
            compare(root.bridge.updateState, "current", "update check result")
            root.bridge.openDataFolder()
            compare(root.bridge.lastAction, "openDataFolder", "folder opened")
            root.bridge.copyLog()
            verify(root.bridge.copiedText.indexOf("[TEST log entry]") >= 0, "log copied")
            verify(root.bridge.copiedText.indexOf("not-a-real-password") < 0, "no password in log")
            root.bridge.copyDiagnostics()
            verify(root.bridge.copiedText.indexOf("not-a-real-password") < 0, "no password in diagnostics")
            root.bridge.setReducedMotion(true)
            compare(root.bridge.reducedMotion, true, "reduced motion set")
            root.bridge.revokeToken()
            compare(root.bridge.lastAction, "revokeToken", "token revoke reported")
        }

        function test_operation_finished_signal() {
            root.lastSignal = ""
            root.bridge.chooseExecutable()
            verify(root.lastSignal.indexOf("Test executable selected.") >= 0, "operation signal delivered")
        }
    }
}
