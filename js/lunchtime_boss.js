import m from "mithril"
import { LunchtimeCell, ClassForCount } from "./lunchtime_cell.js"
import { EmailAddress } from "./email.js"

const allGames = ["A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P"]
let data = window.DATA
data.historicalParticipants = data.historicalParticipants.map(e => EmailAddress.fromEmail(e))
data.participants.forEach(p => {
    p.address = EmailAddress.fromEmail(p.address)
    let i = data.historicalParticipants.findIndex(e => e.equals(p.address))
    if (i == -1) {
        data.historicalParticipants.push(p.address)
    } else {
        data.historicalParticipants[i] = p.address
    }
})

class LunchtimeBoss {
    get showInvite() {
        return data.state == "pending" || data.state == "no_invite_sent"
    }

    get showNoInvite() {
        return data.state == "pending"
    }

    get showBadger() {
        return data.state == "invite_sent"
    }

    get showGameOn() {
        return data.state == "invite_sent" || data.state == "no_game_sent" || data.state == "no_invite_sent"
    }

    get showNoGame() {
        return data.state == "invite_sent" || data.state == "game_on_sent" || data.state == "no_invite_sent" || data.state == "reminder_sent"
    }


    oninit() {
        this.additionalContent = ""
        this.gameOnAdjustedTime = data.gameOnAdjustedTime
        this.gameOnGameKey = data.gameOnGameKey
        this.selectedMessage = null
    }

    playersForGame(key) {
        return data.participants.filter(p => p.gameKeys.includes(key)).map(p => p.address.fullName)
    }

    countForGame(key) {
        return this.playersForGame(key).length
    }

    get currentParticipant() {
        if (!this.currentParticipantEmailIsValid) return null
        let p = data.participants.find(p => p.address.equals(this.currentParticipantEmail))
        if (!p) {
            p = { gameKeys: [] }
            data.participants.push(p)
        }
        p.address = this.currentParticipantEmail
        return p
    }

    game(key) {
        return data.games.find(g => g.key == key)
    }

    selectedByCurrentParticipant(key) {
        if (!this.currentParticipant) return false
        return this.currentParticipant.gameKeys.includes(key)
    }

    toggleGameForCurrentPlayer(key) {
        let p = this.currentParticipant
        if (!p) return
        let i = p.gameKeys.indexOf(key)
        if (i >= 0) {
            p.gameKeys.splice(i, 1)
        } else {
            p.gameKeys.push(key)
        }
    }

    toggleSelectedMessage(message) {
        this.selectedMessage = (this.selectedMessage == message) ? null : message
    }

    get showMessageSendForm() {
        return !!this.selectedMessage
    }

    get enableMessageSendButton() {
        if (this.selectedMessage == "Game On") return !!this.gameOnGameKey
        return this.showMessageSendForm
    }

    get showGamePicker() {
        return this.selectedMessage == "Game On"
    }

    get historicalParticipants() {
        return data.historicalParticipants
    }

    get currentParticipantEmailIsValid() {
        if (!this.currentParticipantEmail) return false
        return this.currentParticipantEmail.hasExplicitName && this.currentParticipantEmail.isValid
    }

    get currentParticipantEmailIsNew() {
        if (!this.currentParticipantEmail) return false
        if (this.historicalParticipants.some(e => e.equals(this.currentParticipantEmail))) return false
        return this.currentParticipantEmailIsValid
    }

    sendMessage() {
        this.successSendMessage = ""
        this.failureSendMessage = ""
        let body = {
            additionalContent: this.additionalContent
        }
        if (this.selectedMessage == "Invite") {
            body.commandType = "admin_invite"
        } else if (this.selectedMessage == "No Invite") {
            body.commandType = "admin_no_invite"
        } else if (this.selectedMessage == "Badger") {
            body.commandType = "admin_badger"
        } else if (this.selectedMessage == "Game On") {
            body.commandType = "admin_game_on"
            body.gameOnGameKey = this.gameOnGameKey
            body.gameOnAdjustedTime = this.gameOnAdjustedTime
        } else if (this.selectedMessage == "No Game") {
            body.commandType = "admin_no_game"
        }

        m.request({
            method: "POST",
            url: "/lunchtime/" + data.bossGuid,
            body: body,
        }).then((res) => {
            this.successSendMessage = "Got it, thanks! Reloading..."
            setTimeout(() => {
                location.reload()
            }, 1000);
        }).catch((err) => {
            this.failureSendMessage = "Whoops, something went wrong. Please try again later."
        })
    }

    submitGames() {
        this.successSetGamesMessage = ""
        this.failureSetGamesMessage = ""
        m.request({
            method: "POST",
            url: "/lunchtime/" + data.bossGuid,
            body: {
                commandType: "set_games",
                participant: this.currentParticipant,
            },
        }).then((res) => {
            this.successSetGamesMessage = "Got it, thanks! Reloading..."
            setTimeout(() => {
                location.reload()
            }, 1000);
        }).catch((err) => {
            this.failureSetGamesMessage = "Whoops, something went wrong. Please try again later."
        })

    }

    dayCell(key) {
        return m(LunchtimeCell, {
            game: this.game(key),
            players: this.playersForGame(key),
            selected: this.selectedByCurrentParticipant(key),
            onclick: () => {
                if (this.currentParticipantEmailIsValid) {
                    this.toggleGameForCurrentPlayer(key)
                } else {
                    document.querySelector("#participant-address").focus()
                }
            },
        })
    }

    view() {
        let currentStateMessage = ""
        if (data.gameOnGameKey) {
            let winner = data.games.find(g => g.key == data.gameOnGameKey)
            currentStateMessage = `Game at: ${winner.day} at ${data.gameOnAdjustedTime ? data.gameOnAdjustedTime : winner.time}`
        }
        return [
            m("h2", "🅱️ ", m("span.green", "Lunchtime"), " (week of ", data.weekOf, ")"),
            m("h3", "Current State: ", m("span.bold.green", data.state.toUpperCase()), m("span.bold", ` `)),
            data.gameOnGameKey && m("h3", `Game On: ${data.gameOnGameFullStartTime}`),
            m("h3", "Send a Message"),
            // a row of buttons to select the kind of message to send
            m(".info", "I want to..."),
            m(".button-row",
                this.showInvite && m("button", {
                    class: this.selectedMessage && this.selectedMessage != "Invite" ? "dim" : "",
                    onclick: () => this.toggleSelectedMessage("Invite")
                }, "Send the Invite"),
                this.showNoInvite && m("button.red", {
                    class: this.selectedMessage && this.selectedMessage != "No Invite" ? "dim" : "",
                    onclick: () => this.toggleSelectedMessage("No Invite")
                }, "Send the NO Invite"),
                this.showBadger && m("button.blue", {
                    class: this.selectedMessage && this.selectedMessage != "Badger" ? "dim" : "",
                    onclick: () => this.toggleSelectedMessage("Badger")
                }, "Send a Badger"),
                this.showGameOn && m("button", {
                    class: this.selectedMessage && this.selectedMessage != "Game On" ? "dim" : "",
                    onclick: () => this.toggleSelectedMessage("Game On")
                }, "Send Game On"),
                this.showNoGame && m("button.red", {
                    class: this.selectedMessage && this.selectedMessage != "No Game" ? "dim" : "",
                    onclick: () => this.toggleSelectedMessage("No Game")
                }, "Send No Game"),
            ),

            this.showMessageSendForm && m("textarea#additional-content.full-width", {
                placeholder: "Additional Content (optional)\nThis is sent on top of the canned message.",
                rows: 3,
                maxLength: 1000,
                value: this.additionalContent,
                onchange: (e) => {
                    this.additionalContent = e.target.value
                }
            }),
            this.showGamePicker && m(".info.bold", "Choose the winning game:"),
            this.showGamePicker && m(".button-row",
                data.games.map(game => {
                    let count = this.countForGame(game.key)
                    return m(".game-option", {
                        id: game.key,
                        onclick: () => this.gameOnGameKey = game.key,
                        class: ClassForCount(count) + (this.gameOnGameKey == game.key ? " selected" : ""),
                    },
                        m(".day", game.day),
                        m(".time", game.time),
                        m(".count", count),
                    )
                })
            ),
            this.showGamePicker && m("input#override-start-time.full-width", {
                type: "text",
                placeholder: "Override Start Time (optional).  Use this to pick out half-times",
                value: this.gameOnAdjustedTime,
                onchange: (e) => {
                    this.gameOnAdjustedTime = e.target.value
                }
            }),
            this.successSendMessage ? m(".message.success.full-width", this.successSendMessage) : null,
            this.failureSendMessage ? m(".message.failure.full-width", this.failureSendMessage) : null,
            this.showMessageSendForm && m(".button-row",
                m("button", {
                    disabled: !this.enableMessageSendButton,
                    onclick: () => this.sendMessage(),
                }, "Send " + this.selectedMessage),
            ),
            m("h3", "Manage Players"),
            m(".pcs",
                data.participants.map(p => m(".pc",
                    {
                        class: this.currentParticipantEmail && this.currentParticipantEmail.equals(p.address) ? "selected" : "",
                        onclick: () => {
                            if (this.currentParticipantEmail && this.currentParticipantEmail.equals(p.address)) {
                                this.currentParticipantEmail = null
                            } else {
                                this.currentParticipantEmail = p.address
                            }
                        }
                    },
                    m(".pc-name", p.address.fullName),
                    m(".pc-email", p.address.address),
                    m(".pc-games",
                        allGames.map(key => m(".pc-game", { class: p.gameKeys.includes(key) && "active" })),
                    ),
                    !!p.comments && m(".pc-comment", p.comments),
                )),
            ),

            m("input#participant-address.full-width", {
                type: "text",
                list: "historical-participants",
                class: (this.currentParticipantEmailIsValid === false) ? "invalid" : "",
                value: this.currentParticipantEmail ? this.currentParticipantEmail.email : "",
                placeholder: "Participant: First Last <email@example.com>",
                onchange: (e) => this.currentParticipantEmail = EmailAddress.fromEmail(e.target.value),
            },
                m("datalist#historical-participants",
                    this.historicalParticipants.map(p => m("option", { value: p.string }))
                ),
            ),
            (this.currentParticipantEmailIsValid === false) && m(".validation-error", "Invalid Participant"),
            this.currentParticipantEmailIsNew && m(".info", "This is a new participant.  They will be added to the list when you submit."),

            m("table.games",
                m("tr", m("th.date", { colspan: 4 }, this.game("A").date)),
                m("tr", ["A", "B", "C", "D"].map(key => this.dayCell(key))),
                m("tr", m("th.date", { colspan: 4 }, this.game("E").date)),
                m("tr", ["E", "F", "G", "H"].map(key => this.dayCell(key))),
                m("tr", m("th.date", { colspan: 4 }, this.game("I").date)),
                m("tr", ["I", "J", "K", "L"].map(key => this.dayCell(key))),
                m("tr", m("th.date", { colspan: 4 }, this.game("M").date)),
                m("tr", ["M", "N", "O", "P"].map(key => this.dayCell(key))),
            ),

            m("textarea#comments", {
                placeholder: "Comments (optional)",
                rows: 3,
                maxLength: 1000,
                disabled: !this.currentParticipantEmailIsValid,
                value: this.currentParticipantEmailIsValid ? this.currentParticipant.comments : "",
                onchange: (e) => {
                    if (this.currentParticipantEmailIsValid) {
                        this.currentParticipant.comments = e.target.value
                    }
                }
            }),
            this.successSetGamesMessage ? m(".message.set-games.success.full-width", this.successSetGamesMessage) : null,
            this.failureSetGamesMessage ? m(".message.set-games.failure.full-width", this.failureSetGamesMessage) : null,
            m("button.submit.full-width", {
                disabled: !this.currentParticipantEmailIsValid,
                onclick: () => this.submitGames(),
            }, "Submit"),

        ]
    }
}

m.mount(document.querySelector("#content"), LunchtimeBoss)