import QtQuick 2.15
import QtTest 1.2
import "../components" as C

Item {
    id: root
    width: 500
    height: 240

    C.Button { id: first; x: 0; y: 0; text: "First" }
    C.IconButton { id: second; x: 120; y: 0; glyph: "+"; label: "Second" }
    C.Checkbox { id: third; x: 200; y: 0; label: "Third" }
    C.Input { id: fourth; x: 300; y: 0; label: "Fourth"; width: 160 }

    C.Button { id: dialogTrigger; x: 0; y: 80; text: "Open dialog" }
    C.Dialog {
        id: dialog
        open: false
        title: "Focus test dialog"
        trigger: dialogTrigger
        focusTargets: [cancel, confirm]
        C.Button { id: cancel; text: "Cancel"; variant: "quiet" }
        C.Button { id: confirm; text: "Confirm" }
    }

    TestCase {
        name: "FocusOrder"
        when: windowShown

        function test_tabOrder() {
            first.forceActiveFocus()
            verify(first.activeFocus)
            keyPress(Qt.Key_Tab)
            tryCompare(second, "activeFocus", true)
            keyPress(Qt.Key_Tab)
            tryCompare(third, "activeFocus", true)
            keyPress(Qt.Key_Tab)
            tryCompare(fourth, "activeFocus", true)
            keyPress(Qt.Key_Tab, Qt.ShiftModifier)
            tryCompare(third, "activeFocus", true)
        }

        function test_dialogTrapAndReturn() {
            dialogTrigger.forceActiveFocus()
            dialog.open = true
            tryCompare(cancel, "activeFocus", true)
            keyPress(Qt.Key_Tab)
            tryCompare(confirm, "activeFocus", true)
            keyPress(Qt.Key_Tab)
            tryCompare(cancel, "activeFocus", true)
            keyPress(Qt.Key_Escape)
            compare(dialog.open, false)
            tryCompare(dialogTrigger, "activeFocus", true)
        }
    }
}
