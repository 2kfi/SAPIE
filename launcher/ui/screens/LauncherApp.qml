import QtQuick 2.15
import QtQuick.Window 2.15
import ".." as UI
import "../components" as C

Item {
    id: app
    property var bridge: UI.FakeCoreBridge
    property string currentScreen: "library"
    property bool reducedMotion: app.bridge.reducedMotion

    function goTo(name) {
        if (app.bridge.connectionState !== "signedIn" && name !== "signin") {
            app.notify("Sign in before opening that screen.")
            app.currentScreen = "signin"
            return
        }
        app.currentScreen = name
    }

    function notify(message) {
        toast.message = message
        toast.shown = true
    }

    function confirm(title, message, action) {
        confirmDialog.title = title
        confirmText.text = message
        confirmDialog.action = action
        confirmDialog.trigger = Window.window ? Window.window.activeFocusItem : null
        confirmDialog.open = true
    }

    Component.onCompleted: {
        if (app.bridge.libraryState === "loading") app.bridge.listGames()
        if (app.bridge.friendsState === "loading") app.bridge.listFriends()
        if (app.bridge.settingsState === "loading") app.bridge.settingsState = "ready"
        if (app.bridge.logsState === "loading") app.bridge.logsState = "ready"
        if (!app.reducedMotion) {
            stage.opacity = 0
            entrance.start()
        }
    }

    Connections {
        target: app.bridge
        function onOperationFinished(operation, ok, message) {
            app.notify(message)
        }
    }

    C.AppShell {
        anchors.fill: parent

        sidebar: C.Sidebar {
            id: rail
            anchors.fill: parent

            Item {
                width: 148
                height: 32
                Text {
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    text: "SAPIE"
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsLg
                    font.weight: Font.Bold
                    Accessible.role: Accessible.StaticText
                    Accessible.name: "SAPIE"
                }
            }
            C.SidebarGroup {
                title: "Library"
                C.SidebarItem {
                    objectName: "navLibrary"
                    text: "Library"
                    glyph: "L"
                    height: 44
                    selected: app.currentScreen === "library" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("library")
                }
                C.SidebarItem {
                    objectName: "navAddGame"
                    text: "Add game"
                    glyph: "+"
                    height: 44
                    selected: app.currentScreen === "add" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("add")
                }
            }
            C.SidebarGroup {
                title: "SAPIE"
                C.SidebarItem {
                    objectName: "navFriends"
                    text: "Friends"
                    glyph: "F"
                    height: 44
                    selected: app.currentScreen === "friends" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("friends")
                }
                C.SidebarItem {
                    objectName: "navRequests"
                    text: "Requests"
                    glyph: "R"
                    height: 44
                    selected: app.currentScreen === "requests" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("requests")
                }
                C.SidebarItem {
                    objectName: "navBlocked"
                    text: "Blocked"
                    glyph: "B"
                    height: 44
                    selected: app.currentScreen === "blocked" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("blocked")
                }
                C.SidebarItem {
                    objectName: "navLobbies"
                    text: "Lobbies"
                    glyph: "O"
                    height: 44
                    selected: app.currentScreen === "lobbies" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("lobbies")
                }
            }
            C.SidebarGroup {
                title: "System"
                C.SidebarItem {
                    objectName: "navSettings"
                    text: "Settings"
                    glyph: "S"
                    height: 44
                    selected: app.currentScreen === "settings" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("settings")
                }
                C.SidebarItem {
                    objectName: "navLogs"
                    text: "Logs"
                    glyph: "G"
                    height: 44
                    selected: app.currentScreen === "logs" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("logs")
                }
                C.SidebarItem {
                    objectName: "navAbout"
                    text: "About"
                    glyph: "A"
                    height: 44
                    selected: app.currentScreen === "about" && app.bridge.connectionState === "signedIn"
                    onClicked: app.goTo("about")
                }
            }
        }

        toolbar: C.Toolbar {
            id: bar
            anchors.fill: parent
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: {
                    if (app.bridge.connectionState !== "signedIn") return "Sign in"
                    if (app.currentScreen === "add") return "Add game"
                    if (app.currentScreen === "game") return "Game"
                    if (app.currentScreen === "friends") return "Friends"
                    if (app.currentScreen === "requests") return "Requests"
                    if (app.currentScreen === "blocked") return "Blocked"
                    if (app.currentScreen === "lobbies") return "Lobbies"
                    if (app.currentScreen === "settings") return "Settings"
                    if (app.currentScreen === "logs") return "Logs"
                    if (app.currentScreen === "about") return "About"
                    return "Library"
                }
                color: UI.Tokens.textPrimary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsSm
                font.weight: Font.DemiBold
                Accessible.role: Accessible.Heading
                Accessible.name: text
            }
            Item { width: UI.Tokens.sp4; height: 1 }
            C.StatusPill {
                anchors.verticalCenter: parent.verticalCenter
                text: app.bridge.connectionState === "signedIn" ? "Connected" : "Not signed in"
                status: app.bridge.connectionState === "signedIn" ? "success" : "offline"
            }
            Item { width: Math.max(UI.Tokens.sp4, bar.width - 360); height: 1 }
            C.Button {
                objectName: "toolbarSignInButton"
                text: app.bridge.connectionState === "signedIn" ? app.bridge.accountName : "Sign in"
                variant: "quiet"
                height: 44
                width: app.bridge.connectionState === "signedIn" ? 140 : 104
                onClicked: app.goTo("signin")
            }
        }

        Item {
            id: stage
            anchors.fill: parent
            NumberAnimation {
                id: entrance
                target: stage
                property: "opacity"
                from: 0
                to: 1
                duration: UI.Tokens.motionHover
                easing.type: Easing.OutCubic
            }

            SignIn {
                objectName: "signInScreen"
                visible: app.bridge.connectionState !== "signedIn"
                anchors.fill: parent
                bridge: app.bridge
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
            }
            Library {
                objectName: "libraryScreen"
                visible: app.bridge.connectionState === "signedIn" && app.currentScreen === "library"
                anchors.fill: parent
                bridge: app.bridge
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
            }
            Game {
                objectName: "gameScreen"
                visible: app.bridge.connectionState === "signedIn" && app.currentScreen === "game"
                anchors.fill: parent
                bridge: app.bridge
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onConfirmRequested: (title, message, action) => app.confirm(title, message, action)
            }
            AddGame {
                objectName: "addGameScreen"
                visible: app.bridge.connectionState === "signedIn" && app.currentScreen === "add"
                anchors.fill: parent
                bridge: app.bridge
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
            }
            Friends {
                objectName: "friendsScreen"
                visible: app.bridge.connectionState === "signedIn"
                         && ["friends", "requests", "blocked", "lobbies"].indexOf(app.currentScreen) >= 0
                anchors.fill: parent
                bridge: app.bridge
                section: app.currentScreen === "friends" ? "friends" : app.currentScreen
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onConfirmRequested: (title, message, action) => app.confirm(title, message, action)
            }
            Settings {
                objectName: "settingsScreen"
                visible: app.bridge.connectionState === "signedIn" && app.currentScreen === "settings"
                anchors.fill: parent
                bridge: app.bridge
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onConfirmRequested: (title, message, action) => app.confirm(title, message, action)
            }
            Logs {
                objectName: "logsScreen"
                visible: app.bridge.connectionState === "signedIn" && app.currentScreen === "logs"
                anchors.fill: parent
                bridge: app.bridge
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
            }
            About {
                objectName: "aboutScreen"
                visible: app.bridge.connectionState === "signedIn" && app.currentScreen === "about"
                anchors.fill: parent
                bridge: app.bridge
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
            }
        }

        C.Dialog {
            id: confirmDialog
            objectName: "confirmDialog"
            property var action: null
            open: false
            title: "Confirm"
            focusTargets: [cancelAction, acceptAction]
            Text {
                id: confirmText
                objectName: "confirmMessage"
                width: parent.width
                color: UI.Tokens.textSecondary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsSm
                wrapMode: Text.Wrap
                Accessible.role: Accessible.StaticText
                Accessible.name: text
            }
            Row {
                anchors.right: parent.right
                spacing: UI.Tokens.sp2
                C.Button {
                    id: cancelAction
                    objectName: "confirmCancel"
                    text: "Cancel"
                    variant: "quiet"
                    height: 44
                    width: 104
                    onClicked: confirmDialog.close()
                }
                C.Button {
                    id: acceptAction
                    objectName: "confirmAccept"
                    text: "Confirm"
                    variant: "danger"
                    height: 44
                    width: 120
                    onClicked: {
                        var action = confirmDialog.action
                        confirmDialog.close()
                        if (action) action()
                    }
                }
            }
        }

        C.Toast {
            id: toast
            objectName: "toast"
            anchors.bottom: parent.bottom
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottomMargin: UI.Tokens.sp4
            width: Math.min(560, parent.width - UI.Tokens.sp8)
            shown: false
            duration: 3200
            z: 10
        }
    }
}
