import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: app

    property string currentScreen: "signin"
    property string demoState: "empty"
    property bool reducedMotion: false
    readonly property var stateModel: ["empty", "loading", "error", "data"]

    function goTo(name) {
        app.currentScreen = name
    }
    function notify(message) {
        toast.message = message
        toast.shown = true
    }
    function retryLoading() {
        app.demoState = "loading"
        retryTimer.start()
    }

    Timer {
        id: retryTimer
        interval: 700
        onTriggered: app.demoState = "data"
    }

    C.AppShell {
        anchors.fill: parent

        sidebar: C.Sidebar {
            anchors.fill: parent
            Item {
                width: 148
                height: 32
                Image {
                    id: brandMark
                    anchors.left: parent.left
                    anchors.verticalCenter: parent.verticalCenter
                    width: 26
                    height: 26
                    fillMode: Image.PreserveAspectFit
                    source: "../../../design/icons/mark-directions/candidate-a-nested.svg"
                    Accessible.role: Accessible.Graphic
                    Accessible.name: "Provisional SAPIE mark, candidate A"
                }
                Text {
                    anchors.left: brandMark.right
                    anchors.leftMargin: UI.Tokens.sp2
                    anchors.verticalCenter: parent.verticalCenter
                    text: "SAPIE"
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsLg
                    font.weight: Font.Bold
                }
            }
            C.SidebarGroup {
                title: "Prototype"
                C.SidebarItem {
                    text: "Sign in"
                    glyph: "S"
                    selected: app.currentScreen === "signin"
                    onClicked: app.goTo("signin")
                }
                C.SidebarItem {
                    text: "Library"
                    glyph: "L"
                    selected: app.currentScreen === "library"
                    onClicked: app.goTo("library")
                }
                C.SidebarItem {
                    text: "Game"
                    glyph: "G"
                    selected: app.currentScreen === "game"
                    onClicked: app.goTo("game")
                }
                C.SidebarItem {
                    text: "Friends"
                    glyph: "F"
                    selected: app.currentScreen === "friends"
                    onClicked: app.goTo("friends")
                }
                C.SidebarItem {
                    text: "Settings"
                    glyph: "T"
                    selected: app.currentScreen === "settings"
                    onClicked: app.goTo("settings")
                }
            }
        }

        toolbar: C.Toolbar {
            anchors.fill: parent
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: app.currentScreen === "signin" ? "Sign in" : (app.currentScreen === "library" ? "Library" : (app.currentScreen === "game" ? "Game" : (app.currentScreen === "friends" ? "Friends" : "Settings")))
                color: UI.Tokens.textPrimary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsSm
                font.weight: Font.DemiBold
            }
            Item { width: UI.Tokens.sp4; height: 1 }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "State"
                color: UI.Tokens.textSecondary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsXs
            }
            C.Select {
                id: stateSelect
                width: 132
                label: "Prototype state"
                model: app.stateModel
                currentIndex: 0
                onCurrentIndexChanged: app.demoState = app.stateModel[currentIndex]
            }
            Text {
                anchors.verticalCenter: parent.verticalCenter
                text: "no live data"
                color: UI.Tokens.textSecondary
                font.family: UI.Tokens.fontMono
                font.pixelSize: UI.Tokens.fsXs
            }
        }

        Item {
            id: contentFrame
            anchors.fill: parent
            opacity: 1
            Component.onCompleted: {
                if (!app.reducedMotion) {
                    contentFrame.opacity = 0
                    entrance.start()
                }
            }
            NumberAnimation {
                id: entrance
                target: contentFrame
                property: "opacity"
                to: 1
                duration: UI.Tokens.motionHover
                easing.type: Easing.OutCubic
            }
            SignIn {
                objectName: "signinScreen"
                visible: app.currentScreen === "signin"
                anchors.fill: parent
                viewState: app.demoState
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onRetryRequested: app.retryLoading()
            }
            Library {
                objectName: "libraryScreen"
                visible: app.currentScreen === "library"
                anchors.fill: parent
                viewState: app.demoState
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onRetryRequested: app.retryLoading()
            }
            Game {
                objectName: "gameScreen"
                visible: app.currentScreen === "game"
                anchors.fill: parent
                viewState: app.demoState
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onRetryRequested: app.retryLoading()
            }
            Friends {
                objectName: "friendsScreen"
                visible: app.currentScreen === "friends"
                anchors.fill: parent
                viewState: app.demoState
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onRetryRequested: app.retryLoading()
            }
            Settings {
                objectName: "settingsScreen"
                visible: app.currentScreen === "settings"
                anchors.fill: parent
                viewState: app.demoState
                onNavigateTo: (name) => app.goTo(name)
                onShowMessage: (message) => app.notify(message)
                onRetryRequested: app.retryLoading()
                onSetReducedMotion: (value) => app.reducedMotion = value
            }
        }

        C.Toast {
            id: toast
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
