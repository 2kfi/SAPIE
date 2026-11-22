import QtQuick 2.15
import "../components" as C
import ".." as UI

Window {
    id: gallery
    visible: true
    width: 1280
    height: 900
    color: UI.Tokens.bgApp
    title: "SAPIE primitives gallery (placeholder data)"

    Flickable {
        anchors.fill: parent
        anchors.margins: UI.Tokens.sp4
        contentWidth: column.implicitWidth
        contentHeight: column.implicitHeight
        clip: true

        Column {
            id: column
            spacing: UI.Tokens.sp5

            Text {
                text: "SAPIE component primitives"
                color: UI.Tokens.textPrimary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fs2xl
            }
            Text {
                text: "Placeholder names and IDs only. No real games, players, lobbies or stats."
                color: UI.Tokens.textSecondary
                font.family: UI.Tokens.fontUi
                font.pixelSize: UI.Tokens.fsSm
            }

            C.Section {
                title: "Shell, sidebar, toolbar, header"
                width: 800
                C.AppShell {
                    width: 800
                    height: 340
                    sidebar: C.Sidebar {
                        anchors.fill: parent
                        C.SidebarGroup {
                            title: "Library"
                            C.SidebarItem { text: "All games"; glyph: "A"; selected: true }
                            C.SidebarItem { text: "Recent"; glyph: "R" }
                        }
                        C.SidebarGroup {
                            title: "SAPIE"
                            C.SidebarItem { text: "Friends"; glyph: "F"; badge: 0 }
                        }
                    }
                    toolbar: C.Toolbar {
                        anchors.fill: parent
                        C.SearchField { label: "Search games" }
                        C.IconButton { glyph: "o"; label: "View options" }
                    }
                    C.PageHeader {
                        width: parent.width
                        title: "Library"
                        subtitle: "Placeholder shell"
                        C.Button { text: "Add game"; width: 104 }
                    }
                    C.EmptyState {
                        anchors.top: parent.top
                        anchors.topMargin: 72
                        anchors.left: parent.left
                        anchors.leftMargin: UI.Tokens.sp6
                        width: 320
                        title: "No games added"
                        message: "Add a game from disk to start a library."
                        C.Button { text: "Add game" }
                    }
                }
            }

            C.Section {
                title: "Navigation primitives, states"
                width: 1000
                Flow { width: 1000; spacing: UI.Tokens.sp3
                    GalleryCell { caption: "SidebarItem default"; C.SidebarItem { text: "Lobbies" } }
                    GalleryCell { caption: "SidebarItem hover"; C.SidebarItem { text: "Lobbies"; demoHover: true } }
                    GalleryCell { caption: "SidebarItem active"; C.SidebarItem { text: "Lobbies"; selected: true } }
                    GalleryCell { caption: "SidebarItem focus"; C.SidebarItem { text: "Lobbies"; demoFocus: true } }
                    GalleryCell { caption: "SidebarItem badge"; C.SidebarItem { text: "Requests"; badge: 2 } }
                    GalleryCell { caption: "SidebarGroup"; cellWidth: 240
                        C.SidebarGroup { title: "System"; C.SidebarItem { text: "Settings" } }
                    }
                    GalleryCell { caption: "Toolbar"; cellWidth: 320
                        C.Toolbar { anchors.fill: parent; C.IconButton { glyph: "<"; label: "Back" } }
                    }
                    GalleryCell { caption: "PageHeader"; cellWidth: 320
                        C.PageHeader { width: parent.width; title: "Friends"; subtitle: "Placeholder header" }
                    }
                }
            }

            C.Section {
                title: "Buttons and inputs"
                width: 1200
                Flow { width: 1200; spacing: UI.Tokens.sp3
                    GalleryCell { caption: "Button primary"; C.Button { text: "Add game" } }
                    GalleryCell { caption: "Button hover"; C.Button { text: "Add game"; demoHover: true } }
                    GalleryCell { caption: "Button focus"; C.Button { text: "Add game"; demoFocus: true } }
                    GalleryCell { caption: "Button disabled"; C.Button { text: "Add game"; enabled: false } }
                    GalleryCell { caption: "Button loading"; C.Button { text: "Add game"; loading: true } }
                    GalleryCell { caption: "Button quiet"; C.Button { text: "Cancel"; variant: "quiet" } }
                    GalleryCell { caption: "Button danger"; C.Button { text: "Remove"; variant: "danger" } }
                    GalleryCell { caption: "PlayButton"; C.PlayButton { gameName: "placeholder" } }
                    GalleryCell { caption: "PlayButton hover"; C.PlayButton { gameName: "placeholder"; demoHover: true } }
                    GalleryCell { caption: "PlayButton focus"; C.PlayButton { gameName: "placeholder"; demoFocus: true } }
                    GalleryCell { caption: "PlayButton launching"; C.PlayButton { gameName: "placeholder"; launching: true } }
                    GalleryCell { caption: "IconButton"; C.IconButton { glyph: "+"; label: "Add" } }
                    GalleryCell { caption: "IconButton hover"; C.IconButton { glyph: "+"; label: "Add"; demoHover: true } }
                    GalleryCell { caption: "IconButton focus"; C.IconButton { glyph: "+"; label: "Add"; demoFocus: true } }
                    GalleryCell { caption: "IconButton danger"; C.IconButton { glyph: "x"; label: "Remove"; variant: "danger" } }
                }
            }

            C.Section {
                title: "Fields"
                width: 1200
                Flow { width: 1200; spacing: UI.Tokens.sp3
                    GalleryCell { caption: "Input"; cellWidth: 260
                        C.Input { label: "Server address"; width: 220 }
                    }
                    GalleryCell { caption: "Input focus"; cellWidth: 260
                        C.Input { label: "Server address"; width: 220; demoFocus: true }
                    }
                    GalleryCell { caption: "Input password"; cellWidth: 260
                        C.Input { label: "Password"; password: true; width: 220 }
                    }
                    GalleryCell { caption: "Input disabled"; cellWidth: 260
                        C.Input { label: "Server address"; width: 220; enabled: false }
                    }
                    GalleryCell { caption: "SearchField"; cellWidth: 260
                        C.SearchField { label: "Search games"; width: 220 }
                    }
                    GalleryCell { caption: "SearchField focus"; cellWidth: 260
                        C.SearchField { label: "Search games"; width: 220; demoFocus: true }
                    }
                    GalleryCell { caption: "Select"; cellWidth: 260
                        C.Select { label: "Sort"; model: ["Name", "Last played"]; width: 160 }
                    }
                    GalleryCell { caption: "Select focus"; cellWidth: 260
                        C.Select { label: "Sort"; model: ["Name", "Last played"]; width: 160; demoFocus: true }
                    }
                    GalleryCell { caption: "Checkbox off"; cellWidth: 220
                        C.Checkbox { label: "Remember me" }
                    }
                    GalleryCell { caption: "Checkbox on"; cellWidth: 220
                        C.Checkbox { label: "Remember me"; checked: true }
                    }
                    GalleryCell { caption: "Checkbox focus"; cellWidth: 220
                        C.Checkbox { label: "Remember me"; demoFocus: true }
                    }
                    GalleryCell { caption: "Checkbox disabled"; cellWidth: 220
                        C.Checkbox { label: "Remember me"; enabled: false }
                    }
                }
            }

            C.Section {
                title: "Library cards and art"
                width: 1100
                Flow { width: 1100; spacing: UI.Tokens.sp3
                    GalleryCell { caption: "MonogramTile seed 42"; cellWidth: 120; cellHeight: 200
                        C.MonogramTile { anchors.fill: parent; appid: 42; label: "[seed 42]" }
                    }
                    GalleryCell { caption: "GameArt local only"; cellWidth: 120; cellHeight: 200
                        C.GameArt { anchors.horizontalCenter: parent.horizontalCenter; width: 104; appid: 7; label: "[cover]" }
                    }
                    GalleryCell { caption: "GameCard default"; cellWidth: 168; cellHeight: 290
                        C.GameCard { title: "[sample A]"; appid: 10 }
                    }
                    GalleryCell { caption: "GameCard installed hover"; cellWidth: 168; cellHeight: 290
                        C.GameCard { title: "[sample B]"; appid: 11; installed: true; demoHover: true }
                    }
                    GalleryCell { caption: "GameCard selected focus"; cellWidth: 168; cellHeight: 290
                        C.GameCard { title: "[sample C]"; appid: 12; selected: true; demoFocus: true }
                    }
                    GalleryCell { caption: "GameGrid"; cellWidth: 360; cellHeight: 310
                        C.GameGrid {
                            columns: 2
                            C.GameCard { title: "[grid 1]"; appid: 20 }
                            C.GameCard { title: "[grid 2]"; appid: 21; installed: true }
                        }
                    }
                    GalleryCell { caption: "StatusPill online"; cellWidth: 180
                        C.StatusPill { text: "Installed"; status: "online" }
                    }
                    GalleryCell { caption: "StatusPill warning"; cellWidth: 180
                        C.StatusPill { text: "Update pending"; status: "warning" }
                    }
                    GalleryCell { caption: "StatusPill offline"; cellWidth: 180
                        C.StatusPill { text: "Offline"; status: "offline" }
                    }
                }
            }

            C.Section {
                title: "People and lobbies"
                width: 1100
                Flow { width: 1100; spacing: UI.Tokens.sp3
                    GalleryCell { caption: "Avatar default"; cellWidth: 120
                        C.Avatar { name: "[sample player]"; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "Avatar small"; cellWidth: 120
                        C.Avatar { name: "[sample player]"; size: 24; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "PresenceDot online"; cellWidth: 140
                        C.PresenceDot { status: "online"; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "PresenceDot away"; cellWidth: 140
                        C.PresenceDot { status: "away"; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "PresenceDot offline"; cellWidth: 140
                        C.PresenceDot { status: "offline"; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "FriendRow default"; cellWidth: 280; cellHeight: 64
                        C.FriendRow { name: "[sample player]"; status: "online" }
                    }
                    GalleryCell { caption: "FriendRow hover"; cellWidth: 280; cellHeight: 64
                        C.FriendRow { name: "[sample player]"; status: "away"; demoHover: true }
                    }
                    GalleryCell { caption: "FriendRow selected"; cellWidth: 280; cellHeight: 64
                        C.FriendRow { name: "[sample player]"; status: "offline"; selected: true }
                    }
                    GalleryCell { caption: "FriendGroup"; cellWidth: 300; cellHeight: 130
                        C.FriendGroup { title: "Online"; C.FriendRow { name: "[sample player]"; status: "online" } }
                    }
                    GalleryCell { caption: "RequestRow incoming"; cellWidth: 380; cellHeight: 72
                        C.RequestRow { name: "[sample player]"; direction: "incoming" }
                    }
                    GalleryCell { caption: "RequestRow outgoing"; cellWidth: 380; cellHeight: 72
                        C.RequestRow { name: "[sample player]"; direction: "outgoing" }
                    }
                    GalleryCell { caption: "BlockedRow"; cellWidth: 300; cellHeight: 64
                        C.BlockedRow { name: "[sample player]" }
                    }
                    GalleryCell { caption: "LobbyBadge joinable"; cellWidth: 160
                        C.LobbyBadge { lobbyState: "joinable"; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "LobbyBadge full"; cellWidth: 160
                        C.LobbyBadge { lobbyState: "full"; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "LobbyBadge private"; cellWidth: 160
                        C.LobbyBadge { lobbyState: "private"; anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "MemberCount unknown"; cellWidth: 140
                        C.MemberCount { anchors.centerIn: parent }
                    }
                    GalleryCell { caption: "LobbyRow"; cellWidth: 460; cellHeight: 76
                        C.LobbyRow { name: "[sample lobby]"; game: "[appid 0]"; lobbyState: "joinable" }
                    }
                    GalleryCell { caption: "LobbyRow full"; cellWidth: 460; cellHeight: 76
                        C.LobbyRow { name: "[sample lobby]"; game: "[appid 0]"; lobbyState: "full"; demoHover: true }
                    }
                }
            }

            C.Section {
                title: "Overlays"
                width: 1100
                Flow { width: 1100; spacing: UI.Tokens.sp3
                    GalleryCell { caption: "Dialog open"; cellWidth: 460; cellHeight: 280
                        C.Dialog {
                            id: sampleDialog
                            open: true
                            title: "Placeholder dialog"
                            Text { text: "Placeholder body. No action has a server behind it."; color: UI.Tokens.textSecondary; wrapMode: Text.Wrap; width: 380 }
                            focusTargets: [dialogCancel, dialogConfirm]
                            C.Button { id: dialogCancel; text: "Cancel"; variant: "quiet" }
                            C.Button { id: dialogConfirm; text: "Confirm" }
                        }
                    }
                    GalleryCell { caption: "Drawer open"; cellWidth: 460; cellHeight: 280
                        C.Drawer {
                            open: true
                            title: "Placeholder drawer"
                            Text { text: "Placeholder drawer content"; color: UI.Tokens.textPrimary }
                        }
                    }
                    GalleryCell { caption: "Menu open"; cellWidth: 220; cellHeight: 120
                        C.Menu {
                            open: true
                            label: "Placeholder menu"
                            C.Button { text: "View details"; variant: "quiet" }
                            C.Button { text: "Remove"; variant: "danger" }
                        }
                    }
                    GalleryCell { caption: "Toast"; cellWidth: 300
                        C.Toast { anchors.fill: parent; message: "Placeholder toast"; duration: 0 }
                    }
                    GalleryCell { caption: "Tooltip"; cellWidth: 180
                        C.Tooltip { anchors.centerIn: parent; text: "Placeholder tooltip"; shown: true }
                    }
                }
            }

            C.Section {
                title: "Data states and layout"
                width: 1100
                Flow { width: 1100; spacing: UI.Tokens.sp3
                    GalleryCell { caption: "EmptyState"; cellWidth: 300; cellHeight: 130
                        C.EmptyState {
                            title: "No requests"
                            message: "Incoming requests appear here."
                            C.Button { text: "Add friend"; variant: "quiet" }
                        }
                    }
                    GalleryCell { caption: "LoadingSkeleton"; cellWidth: 260; cellHeight: 110
                        C.LoadingSkeleton { anchors.fill: parent; lines: 3 }
                    }
                    GalleryCell { caption: "LoadingSkeleton static"; cellWidth: 260; cellHeight: 110
                        C.LoadingSkeleton { anchors.fill: parent; lines: 3; animate: false }
                    }
                    GalleryCell { caption: "ErrorState"; cellWidth: 320; cellHeight: 130
                        C.ErrorState {
                            width: parent.width
                            height: parent.height
                            title: "Server did not answer"
                            message: "Check the address and try again."
                            showRetry: true
                        }
                    }
                    GalleryCell { caption: "ErrorState no retry"; cellWidth: 320; cellHeight: 130
                        C.ErrorState { width: parent.width; height: parent.height; title: "Server did not answer"; message: "Check the address and try again." }
                    }
                    GalleryCell { caption: "OfflineBanner"; cellWidth: 360
                        C.OfflineBanner { anchors.fill: parent }
                    }
                    GalleryCell { caption: "Divider"; cellWidth: 240
                        C.Divider { anchors.centerIn: parent; width: parent.width - UI.Tokens.sp4 }
                    }
                    GalleryCell { caption: "KeyValue"; cellWidth: 260
                        C.KeyValue { anchors.fill: parent; keyName: "App ID"; value: "[none]" }
                    }
                    GalleryCell { caption: "Section"; cellWidth: 260; cellHeight: 120
                        C.Section { title: "Details"; C.KeyValue { keyName: "Arch"; value: "[unknown]" } }
                    }
                }
            }
        }
    }
}
