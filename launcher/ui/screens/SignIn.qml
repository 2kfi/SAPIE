import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "Sign in"

    property var bridge: UI.FakeCoreBridge
    signal navigateTo(string name)
    signal showMessage(string message)

    C.PageHeader {
        id: header
        width: parent.width
        title: "Sign in"
        subtitle: "Connect this launcher to your server"
    }

    C.EmptyState {
        id: firstRun
        visible: root.bridge.connectionState === "signedOut" && root.bridge.firstRun
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "Connect your server"
        message: "Enter the address of your server to sign in. Your password is never written to disk."
        C.Button {
            objectName: "setupServerButton"
            text: "Set up server"
            height: 44
            onClicked: root.bridge.firstRun = false
        }
    }

    Column {
        id: form
        visible: root.bridge.connectionState === "signedOut" && !root.bridge.firstRun
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp4
        spacing: UI.Tokens.sp3
        width: Math.min(360, parent.width - UI.Tokens.sp8)

        C.Input {
            id: hostField
            objectName: "serverField"
            width: parent.width
            label: "Server host and port"
            text: root.bridge.serverAddress
            onTextChanged: root.bridge.serverAddress = text
        }
        C.Input {
            id: userField
            objectName: "usernameField"
            width: parent.width
            label: "Username"
        }
        C.Input {
            id: passField
            objectName: "passwordField"
            width: parent.width
            label: "Password"
            password: true
            Keys.onReturnPressed: submit.clicked()
        }
        C.Checkbox {
            id: remember
            objectName: "rememberField"
            label: "Remember the session token"
            checked: root.bridge.rememberSession
        }
        C.Button {
            id: submit
            objectName: "signInButton"
            text: "Sign in"
            height: 44
            width: 132
            onClicked: {
                if (hostField.text === "") {
                    root.showMessage("Enter your server address before signing in.")
                    hostField.forceActiveFocus()
                } else if (userField.text === "") {
                    root.showMessage("Enter your username before signing in.")
                    userField.forceActiveFocus()
                } else if (passField.text === "") {
                    root.showMessage("Enter your password before signing in.")
                    passField.forceActiveFocus()
                } else {
                    root.bridge.signIn(hostField.text, userField.text, passField.text, remember.checked)
                }
            }
        }
    }

    Column {
        id: checking
        visible: root.bridge.connectionState === "checking"
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
        C.LoadingSkeleton { width: 360; height: 108; lines: 4 }
        C.Button { text: "Sign in"; height: 44; width: 132; loading: true }
    }

    C.ErrorState {
        id: signInError
        visible: ["badHost", "unreachable", "unauthorized", "serverTooOld"].indexOf(root.bridge.connectionState) >= 0
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 132
        title: {
            if (root.bridge.connectionState === "badHost") return "Address is not valid"
            if (root.bridge.connectionState === "unreachable") return "Server did not answer"
            if (root.bridge.connectionState === "unauthorized") return "Credentials were not accepted"
            return "Server is too old"
        }
        message: {
            if (root.bridge.connectionState === "badHost") return "Use a host and port, for example 192.168.1.10:8080."
            if (root.bridge.connectionState === "unreachable") return "Check the address and that your server is running."
            if (root.bridge.connectionState === "unauthorized") return "Check the username and password, then try again."
            return "Update your server to this launcher's version, then try again."
        }
        showRetry: true
        onRetried: {
            root.bridge.connectionState = "signedOut"
            root.bridge.firstRun = false
        }
    }

    C.EmptyState {
        id: signedIn
        visible: root.bridge.connectionState === "signedIn"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(440, parent.width - UI.Tokens.sp8)
        title: "Signed in"
        message: "Connected to " + root.bridge.serverAddress + " as " + root.bridge.accountName + "."
        C.Button {
            objectName: "continueToLibraryButton"
            text: "Open library"
            height: 44
            width: 140
            onClicked: root.navigateTo("library")
        }
    }
}
