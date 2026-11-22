pragma ComponentBehavior: Bound
import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "Friends and social"

    property var bridge: UI.FakeCoreBridge
    property string section: "friends"
    property string searchText: ""
    property string selectedFriendId: ""
    readonly property string pageTitle: section === "requests" ? "Requests"
                                        : section === "blocked" ? "Blocked"
                                        : section === "lobbies" ? "Lobbies" : "Friends"

    signal navigateTo(string name)
    signal showMessage(string message)
    signal confirmRequested(string title, string message, var action)

    function matches(name) {
        return root.searchText === "" || name.toLowerCase().indexOf(root.searchText.toLowerCase()) >= 0
    }
    function inStatus(status) {
        var out = []
        for (var i = 0; i < root.bridge.friends.length; i++) {
            var friend = root.bridge.friends[i]
            if (friend.status === status && matches(friend.name)) out.push(friend)
        }
        return out
    }

    C.PageHeader {
        id: header
        width: parent.width
        title: root.pageTitle
        subtitle: root.section === "lobbies" ? "Sessions from your server" : "From your server"
        C.Button {
            id: addFriendButton
            objectName: "addFriendButton"
            visible: root.section === "friends"
            text: "Add friend"
            height: 44
            width: 128
            onClicked: addDialog.open = true
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
            spacing: UI.Tokens.sp3

            C.SearchField {
                id: search
                objectName: "friendSearch"
                width: Math.min(240, body.width)
                label: "Search " + root.pageTitle.toLowerCase()
                text: root.searchText
                onTextChanged: root.searchText = text
            }

            Row {
                objectName: "friendsOfflineBar"
                visible: root.bridge.friendsState === "offline"
                width: parent.width
                spacing: UI.Tokens.sp2
                C.OfflineBanner {
                    objectName: "friendsOfflineBanner"
                    width: parent.width - 128
                    message: "Server offline. Presence and requests are unavailable."
                }
                C.Button {
                    objectName: "friendsOfflineRetry"
                    text: "Retry"
                    variant: "quiet"
                    height: 44
                    width: 112
                    onClicked: root.section === "lobbies" ? root.bridge.listLobbies() : root.bridge.listFriends()
                }
            }

            Column {
                visible: root.bridge.friendsState === "loading"
                spacing: UI.Tokens.sp3
                Text {
                    text: "Reading " + root.pageTitle.toLowerCase() + "..."
                    color: UI.Tokens.textPrimary
                    font.family: UI.Tokens.fontUi
                    font.pixelSize: UI.Tokens.fsMd
                }
                C.LoadingSkeleton { width: Math.min(420, body.width); height: 108; lines: 4 }
            }

            C.ErrorState {
                id: error
                visible: root.bridge.friendsState === "error"
                width: Math.min(460, body.width)
                height: 116
                title: root.pageTitle + " are unavailable"
                message: "Your server did not answer. Check the connection and try again."
                showRetry: true
                onRetried: root.section === "lobbies" ? root.bridge.listLobbies() : root.bridge.listFriends()
            }

            C.EmptyState {
                id: empty
                visible: root.bridge.friendsState === "empty" || root.bridge.friendsState === "ready"
                         && root.visibleCount === 0
                width: Math.min(440, body.width)
                title: {
                    if (root.section === "requests") return "No requests"
                    if (root.section === "blocked") return "No blocked accounts"
                    if (root.section === "lobbies") return "No lobbies"
                    return "No friends added"
                }
                message: {
                    if (root.section === "requests") return "Friend requests appear here when someone adds you."
                    if (root.section === "blocked") return "Accounts you block appear here."
                    if (root.section === "lobbies") return "Joinable lobbies on your server appear here."
                    return "Add a friend by name to see presence here."
                }
                C.Button {
                    objectName: "emptyAddFriendButton"
                    visible: root.section === "friends"
                    text: "Add friend"
                    height: 44
                    width: 128
                    onClicked: addDialog.open = true
                }
            }

            Column {
                id: friendsData
                visible: root.section === "friends" && root.bridge.friendsState === "ready" && root.visibleCount > 0
                width: parent.width
                spacing: UI.Tokens.sp4
                C.FriendGroup {
                    width: parent.width
                    title: "Online"
                    Repeater {
                        model: root.inStatus("online")
                        delegate: C.FriendRow {
                            required property var modelData
                            name: modelData.name
                            status: "online"
                            note: modelData.note || ""
                            onClicked: {
                                root.selectedFriendId = modelData.id
                                friendDialog.open = true
                            }
                        }
                    }
                }
                C.FriendGroup {
                    width: parent.width
                    title: "Away"
                    Repeater {
                        model: root.inStatus("away")
                        delegate: C.FriendRow {
                            required property var modelData
                            name: modelData.name
                            status: "away"
                            onClicked: {
                                root.selectedFriendId = modelData.id
                                friendDialog.open = true
                            }
                        }
                    }
                }
                C.FriendGroup {
                    width: parent.width
                    title: "Offline"
                    Repeater {
                        model: root.inStatus("offline")
                        delegate: C.FriendRow {
                            required property var modelData
                            name: modelData.name
                            status: "offline"
                            onClicked: {
                                root.selectedFriendId = modelData.id
                                friendDialog.open = true
                            }
                        }
                    }
                }
            }

            Column {
                id: requestsData
                visible: root.section === "requests" && root.bridge.friendsState === "ready" && root.visibleCount > 0
                width: parent.width
                spacing: UI.Tokens.sp2
                Repeater {
                    model: root.bridge.requests
                    delegate: C.RequestRow {
                        required property var modelData
                        visible: root.matches(modelData.name)
                        name: modelData.name
                        direction: modelData.direction
                        onAccepted: {
                            root.bridge.acceptRequest(modelData.id)
                            root.showMessage("Request accepted for " + modelData.name + ".")
                        }
                        onDeclined: {
                            if (modelData.direction === "incoming") root.bridge.declineRequest(modelData.id)
                            else root.bridge.cancelRequest(modelData.id)
                            root.showMessage("Request removed for " + modelData.name + ".")
                        }
                    }
                }
            }

            Column {
                id: blockedData
                visible: root.section === "blocked" && root.bridge.friendsState === "ready" && root.visibleCount > 0
                width: parent.width
                spacing: UI.Tokens.sp2
                Repeater {
                    model: root.bridge.blocked
                    delegate: C.BlockedRow {
                        required property var modelData
                        visible: root.matches(modelData.name)
                        name: modelData.name
                        onUnblocked: root.confirmRequested("Unblock account",
                            "Unblock " + modelData.name + "? They can send you a request again.",
                            function () { root.bridge.unblockFriend(modelData.id) })
                    }
                }
            }

            Column {
                id: lobbyData
                visible: root.section === "lobbies" && root.bridge.friendsState === "ready" && root.visibleCount > 0
                width: parent.width
                spacing: UI.Tokens.sp2
                Repeater {
                    model: root.bridge.lobbies
                    delegate: C.LobbyRow {
                        required property var modelData
                        visible: root.matches(modelData.name)
                        name: modelData.name
                        game: modelData.game || ""
                        members: modelData.members === undefined ? -1 : modelData.members
                        capacity: modelData.capacity === undefined ? -1 : modelData.capacity
                        lobbyState: modelData.lobbyState || "joinable"
                        onJoin: {
                            root.bridge.joinLobby(modelData.id)
                            root.showMessage("Join requested for " + modelData.name + ".")
                        }
                    }
                }
            }
        }
    }

    property int visibleCount: {
        if (root.section === "friends") return root.bridge.friends.length
        if (root.section === "requests") return root.bridge.requests.length
        if (root.section === "blocked") return root.bridge.blocked.length
        return root.bridge.lobbies.length
    }

    C.Dialog {
        id: addDialog
        objectName: "addFriendDialog"
        title: "Add friend"
        trigger: addFriendButton
        focusTargets: [cancelAdd, confirmAdd]
        C.Input {
            id: friendName
            objectName: "newFriendField"
            width: parent.width
            label: "Player name or ID"
        }
        Text {
            width: parent.width
            text: "The request goes to your server. No account is created here."
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsXs
            wrapMode: Text.Wrap
            Accessible.role: Accessible.StaticText
            Accessible.name: text
        }
        Row {
            anchors.right: parent.right
            spacing: UI.Tokens.sp2
            C.Button {
                id: cancelAdd
                objectName: "addFriendCancel"
                text: "Cancel"
                variant: "quiet"
                height: 44
                width: 104
                onClicked: addDialog.close()
            }
            C.Button {
                id: confirmAdd
                objectName: "addFriendConfirm"
                text: "Add friend"
                height: 44
                width: 128
                onClicked: {
                    if (friendName.text === "") {
                        root.showMessage("Enter a player name or ID before sending the request.")
                        friendName.forceActiveFocus()
                        return
                    }
                    root.bridge.addFriend(friendName.text)
                    addDialog.close()
                    root.showMessage("Request sent to your server.")
                }
            }
        }
    }

    C.Dialog {
        id: friendDialog
        objectName: "friendActionsDialog"
        title: "Friend actions"
        focusTargets: [inviteButton, removeButton, blockButton, cancelFriend]
        Text {
            width: parent.width
            text: "Actions apply to your server, not to this launcher's library."
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsXs
            wrapMode: Text.Wrap
            Accessible.role: Accessible.StaticText
            Accessible.name: text
        }
        Flow {
            width: parent.width
            spacing: UI.Tokens.sp2
            C.Button {
                id: inviteButton
                objectName: "inviteFriendButton"
                text: "Invite to lobby"
                variant: "quiet"
                height: 44
                width: 148
                onClicked: {
                    root.bridge.inviteFriend(root.selectedFriendId, "")
                    friendDialog.close()
                    root.showMessage("Invite sent through your server.")
                }
            }
            C.Button {
                id: removeButton
                objectName: "removeFriendButton"
                text: "Remove"
                variant: "danger"
                height: 44
                width: 112
                onClicked: {
                    friendDialog.close()
                    root.confirmRequested("Remove friend",
                        "Remove this friend from your server? You can add them again later.",
                        function () { root.bridge.removeFriend(root.selectedFriendId) })
                }
            }
            C.Button {
                id: blockButton
                objectName: "blockFriendButton"
                text: "Block"
                variant: "danger"
                height: 44
                width: 104
                onClicked: {
                    friendDialog.close()
                    root.confirmRequested("Block account",
                        "Block this account on your server? They will not be able to add you.",
                        function () { root.bridge.blockFriend(root.selectedFriendId) })
                }
            }
            C.Button {
                id: cancelFriend
                text: "Cancel"
                variant: "quiet"
                height: 44
                width: 104
                onClicked: friendDialog.close()
            }
        }
    }
}
