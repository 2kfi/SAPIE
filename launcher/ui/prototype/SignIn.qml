import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    property string viewState: "empty"
    signal navigateTo(string name)
    signal showMessage(string message)
    signal retryRequested()

    C.PageHeader {
        id: header
        width: parent.width
        title: "Sign in"
        subtitle: "Connect this launcher to your server"
    }

    C.EmptyState {
        id: empty
        visible: root.viewState === "empty"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "No server saved"
        message: "Enter your server address to sign in."
        C.Button {
            id: exampleButton
            text: "Use example address"
            width: 168
            onClicked: {
                serverField.text = "example.local:8080"
                root.showMessage("Example address filled in. No connection was made.")
            }
        }
    }

    Column {
        id: checking
        visible: root.viewState === "loading"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        spacing: UI.Tokens.sp3
        Text {
            text: "Checking your server..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton {
            width: 360
            height: 108
            lines: 4
        }
        C.Button { text: "Sign in"; loading: true }
    }

    C.ErrorState {
        id: signInError
        visible: root.viewState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 112
        title: "Server did not answer"
        message: "Check the address and try again. This prototype sends no request."
        showRetry: true
        onRetried: root.retryRequested()
    }

    Column {
        id: form
        visible: root.viewState === "data"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp4
        spacing: UI.Tokens.sp3
        Text {
            text: "Prototype values. No request is sent and no password is stored."
            color: UI.Tokens.textSecondary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsSm
        }
        C.Input { id: serverField; label: "Server address"; width: 320 }
        C.Input { id: userField; label: "Username"; width: 320 }
        C.Input { id: passField; label: "Password"; width: 320; password: true }
        C.Checkbox { id: remember; label: "Remember the session token" }
        C.Button {
            text: "Sign in"
            width: 120
            onClicked: {
                if (serverField.text === "" || userField.text === "" || passField.text === "") {
                    root.showMessage("Enter a server address, a username and a password to continue.")
                    return
                }
                root.navigateTo("library")
                root.showMessage("Prototype: signed in. No server was contacted.")
            }
        }
    }
}
