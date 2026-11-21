import QtQuick 2.15
import QtTest 1.2

Item {
    id: root
    width: 1280
    height: 800
    property var app: null

    TestCase {
        name: "IdentityPrototype"
        when: windowShown

        function initTestCase() {
            var component = Qt.createComponent(Qt.resolvedUrl("PrototypeApp.qml"))
            compare(component.status, Component.Ready, "prototype component loads")
            root.app = component.createObject(root, { width: 1280, height: 800 })
            verify(root.app !== null, "prototype object exists")
            wait(200)
        }

        function cleanupTestCase() {
            if (root.app)
                root.app.destroy()
        }

        function test_sidebar_screens() {
            var names = ["signin", "library", "game", "friends", "settings"]
            for (var i = 0; i < names.length; i++) {
                root.app.currentScreen = names[i]
                compare(findChild(root.app, names[i] + "Screen").visible, true, names[i] + " visible")
            }
        }

        function test_each_screen_state() {
            var screens = ["signin", "library", "game", "friends", "settings"]
            var states = ["empty", "loading", "error", "data"]
            for (var i = 0; i < screens.length; i++) {
                root.app.currentScreen = screens[i]
                for (var j = 0; j < states.length; j++) {
                    root.app.demoState = states[j]
                    compare(findChild(root.app, screens[i] + "Screen").viewState, states[j], screens[i] + " " + states[j])
                }
            }
            root.app.currentScreen = "signin"
            root.app.demoState = "empty"
        }

        function test_dialog_focus_trap_and_escape() {
            root.app.currentScreen = "library"
            root.app.demoState = "empty"
            var trigger = findChild(root.app, "addGameButton")
            var dialog = findChild(root.app, "addGameDialog")
            var cancel = findChild(dialog, "dialogCancel")
            var confirm = findChild(dialog, "dialogConfirm")
            trigger.clicked()
            tryCompare(dialog, "open", true)
            tryVerify(function() { return cancel.activeFocus }, 2000)
            keyPress(Qt.Key_Tab)
            tryVerify(function() { return confirm.activeFocus }, 2000)
            keyPress(Qt.Key_Tab)
            tryVerify(function() { return cancel.activeFocus }, 2000)
            keyPress(Qt.Key_Escape)
            tryCompare(dialog, "open", false)
            tryVerify(function() { return trigger.activeFocus }, 2000)
        }

        function test_retry_returns_to_data() {
            root.app.currentScreen = "library"
            root.app.demoState = "error"
            var retry = findChild(root.app, "libraryScreen")
            verify(retry !== null)
            root.app.retryLoading()
            compare(root.app.demoState, "loading")
            tryCompare(root.app, "demoState", "data", 2000)
        }

        function test_reduced_motion_signal() {
            root.app.currentScreen = "settings"
            root.app.demoState = "data"
            var toggle = findChild(root.app, "reducedMotionToggle")
            toggle.checked = true
            toggle.toggled()
            compare(root.app.reducedMotion, true)
            toggle.checked = false
            toggle.toggled()
            compare(root.app.reducedMotion, false)
        }
    }
}
