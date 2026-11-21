import QtQuick 2.15
import ".." as UI

Rectangle {
    id: root
    height: 1
    color: UI.Tokens.borderSubtle
    Accessible.role: Accessible.Separator
    Accessible.name: "Divider"
}
