pragma Singleton
import QtQuick 2.15

// Narrow stand-in for the M23 CoreBridge. All records here are labelled test data.
QtObject {
    readonly property bool isFake: true

    property string connectionState: "signedOut"
    property string serverAddress: ""
    property string accountName: ""
    property bool rememberSession: false
    property bool firstRun: true

    property string libraryState: "loading"
    property var games: [
        { id: "test-1", name: "[PLACEHOLDER game A]", appid: 0, cover: "", installed: false,
          missing: false, favorite: false, lastPlayed: "", playtimeSeconds: -1 },
        { id: "test-2", name: "[PLACEHOLDER game B]", appid: 0, cover: "", installed: true,
          missing: false, favorite: true, lastPlayed: "", playtimeSeconds: -1 },
        { id: "test-3", name: "[PLACEHOLDER game C]", appid: 0, cover: "", installed: false,
          missing: true, favorite: false, lastPlayed: "", playtimeSeconds: -1 }
    ]
    property string gameState: "empty"
    property var game: ({})

    property string addState: "idle"
    property var detection: ({ path: "[not picked]", arch: "[not scanned]", era: "Unknown",
        interfaces: 0, appid: 0, duplicate: false })

    property string friendsState: "loading"
    property var friends: []
    property var requests: []
    property var blocked: []
    property var lobbies: []

    property string settingsState: "loading"
    property string logsState: "loading"
    property var logs: []
    property var prefs: ({ librarySearch: "", libraryFilter: "All games", librarySort: "Name" })
    property string version: "[TEST version]"
    property string updateChannel: "stable"
    property string updateState: "idle"
    property string dataFolder: "[TEST data folder]"

    property string launchState: "idle"
    property string launchError: ""
    property bool reducedMotion: false

    // Fake-only observation hooks for tests. The real bridge has no such properties.
    property string lastAction: ""
    property string copiedText: ""

    signal operationFinished(string operation, bool ok, string message)

    function signIn(host, username, password, remember) {
        lastAction = "signIn"
        if (host === "" || username === "" || password === "") {
            connectionState = "badHost"
            operationFinished("signIn", false, "Server, username and password are required.")
            return
        }
        connectionState = "signedIn"
        serverAddress = host
        accountName = username
        rememberSession = remember
        firstRun = false
        operationFinished("signIn", true, "Signed in to " + host + ".")
    }

    function signOut() {
        lastAction = "signOut"
        connectionState = "signedOut"
        operationFinished("signOut", true, "Signed out. The local token was cleared.")
    }

    function revokeToken() {
        lastAction = "revokeToken"
        copiedText = ""
        operationFinished("revokeToken", true, "Local token revoked.")
    }

    function listGames() {
        lastAction = "listGames"
        libraryState = games.length === 0 ? "empty" : "ready"
        operationFinished("listGames", true, "Library read.")
    }

    function getGame(id) {
        lastAction = "getGame"
        var found = null
        for (var i = 0; i < games.length; i++) {
            if (games[i].id === id) found = games[i]
        }
        if (!found) {
            game = ({})
            gameState = "empty"
            operationFinished("getGame", false, "No game is selected.")
            return
        }
        game = found
        gameState = found.missing ? "missing" : "ready"
        operationFinished("getGame", true, "Game details read.")
    }

    function setFavorite(id, value) {
        lastAction = "setFavorite"
        for (var i = 0; i < games.length; i++) {
            if (games[i].id === id) games[i].favorite = value
        }
        operationFinished("setFavorite", true, value ? "Added to favorites." : "Removed from favorites.")
    }

    function playGame(id) {
        lastAction = "playGame"
        launchState = "launching"
        launchError = ""
        launchState = "running"
        operationFinished("playGame", true, "Launch requested for the test record.")
    }

    function repairGame(id) {
        lastAction = "repairGame"
        operationFinished("repairGame", true, "Library file reinstalled for the test record.")
    }

    function restoreGame(id) {
        lastAction = "restoreGame"
        operationFinished("restoreGame", true, "Original library restored for the test record.")
    }

    function removeGame(id) {
        lastAction = "removeGame"
        operationFinished("removeGame", true, "Record removed from the test library.")
    }

    function saveGameOptions(id, args, preload) {
        lastAction = "saveGameOptions"
        operationFinished("saveGameOptions", true, "Launch options saved for the test record.")
    }

    function chooseExecutable() {
        lastAction = "chooseExecutable"
        detection = { path: "[TEST path]", arch: "x64", era: "Modern", interfaces: 3,
            appid: 0, duplicate: false }
        addState = "detecting"
        operationFinished("chooseExecutable", true, "Test executable selected.")
    }

    function detectGame(path) {
        lastAction = "detectGame"
        addState = "review"
        operationFinished("detectGame", true, "Detection finished for the test path.")
    }

    function installGame(review) {
        lastAction = "installGame"
        if (review && review.duplicate) {
            addState = "duplicate"
            operationFinished("installGame", false, "That executable is already in your library.")
            return
        }
        addState = "installing"
        addState = "done"
        operationFinished("installGame", true, "Test record added to the library.")
    }

    function cancelInstall() {
        lastAction = "cancelInstall"
        addState = "idle"
        operationFinished("cancelInstall", true, "Install cancelled.")
    }

    function rollbackInstall() {
        lastAction = "rollbackInstall"
        addState = "idle"
        operationFinished("rollbackInstall", true, "Test copy rolled back. Original library is in place.")
    }

    function listFriends() {
        lastAction = "listFriends"
        friendsState = friends.length === 0 && requests.length === 0 && blocked.length === 0 ? "empty" : "ready"
        operationFinished("listFriends", true, "Friends read.")
    }

    function addFriend(nameOrId) {
        lastAction = "addFriend"
        operationFinished("addFriend", true, "Request sent for the test account.")
    }

    function removeFriend(id) {
        lastAction = "removeFriend"
        operationFinished("removeFriend", true, "Friend removed from the test list.")
    }

    function blockFriend(id) {
        lastAction = "blockFriend"
        operationFinished("blockFriend", true, "Account blocked in the test list.")
    }

    function unblockFriend(id) {
        lastAction = "unblockFriend"
        operationFinished("unblockFriend", true, "Account unblocked in the test list.")
    }

    function acceptRequest(id) {
        lastAction = "acceptRequest"
        operationFinished("acceptRequest", true, "Request accepted in the test list.")
    }

    function declineRequest(id) {
        lastAction = "declineRequest"
        operationFinished("declineRequest", true, "Request declined in the test list.")
    }

    function cancelRequest(id) {
        lastAction = "cancelRequest"
        operationFinished("cancelRequest", true, "Outgoing request cancelled in the test list.")
    }

    function listLobbies() {
        lastAction = "listLobbies"
        friendsState = lobbies.length === 0 ? "empty" : "ready"
        operationFinished("listLobbies", true, "Lobbies read.")
    }

    function joinLobby(id) {
        lastAction = "joinLobby"
        operationFinished("joinLobby", true, "Join requested for the test lobby.")
    }

    function inviteFriend(id, lobbyId) {
        lastAction = "inviteFriend"
        operationFinished("inviteFriend", true, "Invite sent for the test account.")
    }

    function getPreference(key, fallback) {
        return prefs[key] === undefined ? fallback : prefs[key]
    }

    function setPreference(key, value) {
        lastAction = "setPreference"
        prefs[key] = value
        operationFinished("setPreference", true, "Preference saved.")
    }

    function setUpdateChannel(channel) {
        lastAction = "setUpdateChannel"
        updateChannel = channel
        operationFinished("setUpdateChannel", true, "Update channel set to " + channel + ".")
    }

    function checkForUpdates() {
        lastAction = "checkForUpdates"
        updateState = "current"
        operationFinished("checkForUpdates", true, "No update is available in the test bridge.")
    }

    function openDataFolder() {
        lastAction = "openDataFolder"
        operationFinished("openDataFolder", true, "Test data folder: " + dataFolder)
    }

    function copyLog() {
        lastAction = "copyLog"
        copiedText = logs.map(function (entry) { return entry.message }).join("\n")
        operationFinished("copyLog", true, "Log copied by the test bridge.")
    }

    function copyDiagnostics() {
        lastAction = "copyDiagnostics"
        copiedText = version + " / " + updateChannel + " / " + dataFolder
        operationFinished("copyDiagnostics", true, "Diagnostics copied by the test bridge.")
    }

    function setReducedMotion(value) {
        lastAction = "setReducedMotion"
        reducedMotion = value
        operationFinished("setReducedMotion", true, "Reduced motion updated.")
    }
}
