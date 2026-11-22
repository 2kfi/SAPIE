import QtQuick 2.15
import ".." as UI
import "../components" as C

Item {
    id: root
    Accessible.role: Accessible.Pane
    Accessible.name: "Logs"

    property var bridge: UI.FakeCoreBridge
    property string level: "All"
    readonly property var levels: ["All", "Info", "Warning", "Error"]

    signal navigateTo(string name)
    signal showMessage(string message)

    function filtered() {
        var out = []
        for (var i = 0; i < root.bridge.logs.length; i++) {
            var entry = root.bridge.logs[i]
            if (root.level !== "All" && entry.level !== root.level) continue
            out.push(entry)
        }
        return out
    }

    C.PageHeader {
        id: header
        width: parent.width
        title: "Logs"
        subtitle: "Launcher activity"
        C.Button {
            objectName: "copyLogButton"
            text: "Copy log"
            height: 44
            width: 120
            onClicked: {
                root.bridge.copyLog()
                root.showMessage("Log copied.")
            }
        }
    }

    Column {
        id: loading
        visible: root.bridge.logsState === "loading"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        spacing: UI.Tokens.sp3
        Text {
            text: "Reading activity..."
            color: UI.Tokens.textPrimary
            font.family: UI.Tokens.fontUi
            font.pixelSize: UI.Tokens.fsMd
        }
        C.LoadingSkeleton { width: 420; height: 108; lines: 4 }
    }

    C.ErrorState {
        id: error
        visible: root.bridge.logsState === "error"
        anchors.left: parent.left
        anchors.leftMargin: UI.Tokens.sp6
        anchors.top: header.bottom
        anchors.topMargin: UI.Tokens.sp5
        width: Math.min(460, parent.width - UI.Tokens.sp8)
        height: 116
        title: "Log could not be read"
        message: "The activity file is unreadable. Check permissions and try again."
        showRetry: true
        onRetried: root.bridge.logsState = "ready"
    }

    Column {
        id: content
        visible: root.bridge.logsState === "ready"
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: header.bottom
        anchors.bottom: parent.bottom
        anchors.margins: UI.Tokens.sp4
        spacing: UI.Tokens.sp3

        C.Select {
            id: levelField
            objectName: "logLevel"
            width: 140
            label: "Log level"
            model: root.levels
            currentIndex: 0
            onCurrentIndexChanged: root.level = root.levels[currentIndex]
        }

        C.EmptyState {
            objectName: "logsEmpty"
            width: Math.min(440, parent.width)
            visible: root.bridge.logs.length === 0 || root.filtered().length === 0
            title: root.bridge.logs.length === 0 ? "No activity yet" : "No entries at this level"
            message: root.bridge.logs.length === 0
                     ? "Launcher activity appears here as you use it."
                     : "Choose another level to see the rest of the log."
            C.Button {
                text: "All levels"
                variant: "quiet"
                height: 44
                width: 120
                onClicked: levelField.currentIndex = 0
            }
        }

        Flickable {
            id: scroller
            visible: root.bridge.logs.length > 0 && root.filtered().length > 0
            width: parent.width
            height: parent.height - levelField.height - UI.Tokens.sp3
            contentWidth: rows.implicitWidth
            contentHeight: rows.implicitHeight
            clip: true
            flickableDirection: Flickable.VerticalFlick
            Column {
                id: rows
                width: scroller.width
                spacing: UI.Tokens.sp2
                Repeater {
                    model: root.filtered()
                    delegate: Item {
                        id: row
                        required property var modelData
                        width: parent.width
                        height: 32
                        Accessible.role: Accessible.ListItem
                        Accessible.name: row.modelData.level + ": " + row.modelData.message
                        Text {
                            anchors.left: parent.left
                            anchors.verticalCenter: parent.verticalCenter
                            width: 72
                            text: row.modelData.level
                            color: row.modelData.level === "Error" ? UI.Tokens.danger
                                   : (row.modelData.level === "Warning" ? UI.Tokens.warning : UI.Tokens.textSecondary)
                            font.family: UI.Tokens.fontUi
                            font.pixelSize: UI.Tokens.fsXs
                        }
                        Text {
                            anchors.left: parent.left
                            anchors.leftMargin: 80
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            text: row.modelData.message
                            color: UI.Tokens.textPrimary
                            font.family: UI.Tokens.fontMono
                            font.pixelSize: UI.Tokens.fsXs
                            elide: Text.ElideRight
                        }
                    }
                }
            }
        }
    }
}
