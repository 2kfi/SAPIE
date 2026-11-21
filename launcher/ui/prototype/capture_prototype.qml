import QtQuick 2.15
import QtTest 1.2

Item {
    id: root
    width: 1280
    height: 800
    property var app: null
    property bool saved: false

    TestCase {
        name: "CaptureIdentityPrototype"
        when: windowShown

        function initTestCase() {
            var component = Qt.createComponent(Qt.resolvedUrl("PrototypeApp.qml"))
            compare(component.status, Component.Ready, "prototype component loads")
            root.app = component.createObject(root, { width: 1280, height: 800 })
            verify(root.app !== null, "prototype object exists")
            wait(250)
        }

        function cleanupTestCase() {
            if (root.app)
                root.app.destroy()
        }

        function shot(name) {
            root.saved = false
            root.app.grabToImage(function(result) {
                if (result !== null)
                    root.saved = result.saveToFile(name + ".png")
            })
            tryVerify(function() { return root.saved }, 5000)
            wait(60)
        }

        function test_capture() {
            var combos = [
                ["signin", "empty", "signin-empty"],
                ["signin", "error", "signin-error"],
                ["signin", "data", "signin-data"],
                ["library", "empty", "library-empty"],
                ["library", "loading", "library-loading"],
                ["library", "error", "library-error"],
                ["library", "data", "library-data"],
                ["game", "data", "game-data"],
                ["friends", "data", "friends-data"],
                ["settings", "data", "settings-data"]
            ]
            for (var i = 0; i < combos.length; i++) {
                root.app.currentScreen = combos[i][0]
                root.app.demoState = combos[i][1]
                wait(220)
                shot(combos[i][2])
            }
        }
    }
}
