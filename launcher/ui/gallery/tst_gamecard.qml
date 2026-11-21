import QtQuick 2.15
import QtTest 1.2
import "../components" as C

Item {
    id: root
    width: 300
    height: 400
    property bool played: false
    property bool selected: false
    C.GameCard {
        id: card
        x: 10
        y: 10
        title: "[sample]"
        demoHover: true
        onPlayRequested: root.played = true
        onClicked: root.selected = true
    }
    TestCase {
        name: "GameCardClick"
        when: windowShown
        function test_playButtonClick() {
            mouseClick(card, 40, 278)
            tryCompare(root, "played", true)
            compare(root.selected, false)
        }
    }
}
