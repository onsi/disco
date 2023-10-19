import m from "mithril"
import { LunchtimeCell } from "./lunchtime_cell.js"
import { EmailAddress } from "./email.js"

let name = window.localStorage.getItem("name") || ""
let email = window.localStorage.getItem("email") || ""
let data = window.DATA
data.participants.forEach(p => p.address = EmailAddress.fromEmail(p.address))


class LunchtimePlayer {
    playersForGame(key) {
        return data.participants.filter(p => p.gameKeys.includes(key)).map(p => p.address.name)
    }
    get isValid() {
        return this.isValidName && this.isValidEmail
    }
    get isValidName() {
        return name.trim().length > 0
    }
    get isValidEmail() {
        return EmailAddress.isValidAddress(email)
    }
    get currentParticipant() {
        if (!this.isValid) return null
        let p = data.participants.find(p => p.address.address == email)
        if (!p) {
            p = { gameKeys: [] }
            data.participants.push(p)
        }
        p.address = EmailAddress.fromNameAndAddress(name, email)
        return p
    }
    selectedByCurrentPlayer(key) {
        if (!this.currentParticipant) return false
        return this.currentParticipant.gameKeys.includes(key)
    }
    toggle(key) {
        let p = this.currentParticipant
        if (!p) return
        let i = p.gameKeys.indexOf(key)
        if (i >= 0) {
            p.gameKeys.splice(i, 1)
        } else {
            p.gameKeys.push(key)
        }
    }
    dayCell(key) {
        return m(LunchtimeCell, {
            game: data.games[key],
            players: this.playersForGame(key),
            selected: this.selectedByCurrentPlayer(key),
            onclick: () => {
                if (!this.isValidName) {
                    document.querySelector("#name").focus()
                } else if (!this.isValidEmail) {
                    document.querySelector("#email").focus()
                } else {
                    this.toggle(key)
                }
            },
        })
    }

    submit() {
        this.successMessage = ""
        this.failureMessage = ""
        m.request({
            method: "POST",
            url: "/lunchtime/" + data.guid,
            body: this.currentParticipant,
        }).then((res) => {
            this.successMessage = "Got it, thanks!"
        }).catch((err) => {
            this.failureMessage = "Whoops, something went wrong. Please try again later."
        })
    }

    view() {
        return [
            m("h2", "Sign up for this week's ", m("span.green", "Lunchtime Game"), " (week of ", data.weekOf, ")"),
            data.gameOnGameKey && m("h3", "Game On! ", m("span.green", data.gameOnGameFullStartTime)),
            m(".info", "Give us your name and e-mail address.  You'll only need to do this once per device - we'll remember it for you going forward."),
            m("input#name.full-width", {
                placeholder: "Name",
                type: "text",
                maxlength: "100",
                value: name,
                class: this.isValidName ? "" : "invalid",
                required: true, onchange: (e) => {
                    name = e.target.value
                    window.localStorage.setItem("name", name)
                }
            }),
            this.isValidName ? null : m(".validation-error", "Please enter your name"),
            m("input#email.full-width", {
                placeholder: "E-mail Address",
                type: "email",
                maxlength: "100",
                value: email,
                class: this.isValidEmail ? "" : "invalid",
                required: true, onchange: (e) => {
                    email = e.target.value
                    window.localStorage.setItem("email", email)
                }
            }),
            this.isValidEmail ? null : m(".validation-error", "Please enter a valid e-mail address"),
            this.isValid ? m(".info", "Now, pick the games you can make then hit ", m("span.green.bold", "submit"), " down below.") : null,

            m("table.games",
                m("tr", m("th.date", { colspan: 4 }, data.games["A"].date)),
                m("tr", ["A", "B", "C", "D"].map(key => this.dayCell(key))),
                m("tr", m("th.date", { colspan: 4 }, data.games["E"].date)),
                m("tr", ["E", "F", "G", "H"].map(key => this.dayCell(key))),
                m("tr", m("th.date", { colspan: 4 }, data.games["I"].date)),
                m("tr", ["I", "J", "K", "L"].map(key => this.dayCell(key))),
                m("tr", m("th.date", { colspan: 4 }, data.games["M"].date)),
                m("tr", ["M", "N", "O", "P"].map(key => this.dayCell(key))),
            ),
            m("textarea#comments", {
                placeholder: "Comments (optional)",
                rows: 3,
                maxLength: 1000,
                disabled: !this.isValid,
                value: this.isValid ? this.currentParticipant.comments : "",
                onchange: (e) => {
                    if (this.isValid) {
                        this.currentParticipant.comments = e.target.value
                    }
                }
            }),
            this.successMessage ? m(".message.set-games.success.full-width", this.successMessage) : null,
            this.failureMessage ? m(".message.set-games.failure.full-width", this.failureMessage) : null,
            m("button.submit.full-width", {
                disabled: !this.isValid,
                onclick: () => this.submit(),
            }, "Submit"),
        ]
    }
}

m.mount(document.querySelector("#content"), LunchtimePlayer)