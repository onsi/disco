import m from "mithril"
import { LunchtimeCell } from "./lunchtime_cell.js"

let name = window.localStorage.getItem("name") || ""
let email = window.localStorage.getItem("email") || ""
let data = window.DATA

function NameFromEmailAddress(email) {
    let m = email.match(/(.*) <.*>/)
    if (m) return m[1]
    m = email.match(/.*\s*(.*)@.*/)
    if (m) return m[1]
    return email
}

class LunchtimePlayer {
    playersForGame(key) {
        return data.participants.filter(p => p.gameKeys.includes(key)).map(p => NameFromEmailAddress(p.address))
    }
    get isValid() {
        return this.isValidName && this.isValidEmail
    }
    get isValidName() {
        return name.trim().length > 0
    }
    get isValidEmail() {
        return email.includes("@") && email.includes(".")
    }
    get currentParticipant() {
        if (!this.isValid) return null
        let p = data.participants.find(p => p.address.includes(email))
        if (!p) {
            p = { gameKeys: [] }
            data.participants.push(p)
        }
        p.address = `${name} <${email}>`
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
            this.isValidName ? null : m("div.invalid-name", "Please enter your name"),
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
            this.isValidEmail ? null : m("div.invalid-email", "Please enter a valid e-mail address"),
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
            this.successMessage ? m(".message.success.full-width", this.successMessage) : null,
            this.failureMessage ? m(".message.failure.full-width", this.failureMessage) : null,
            m("button.submit.full-width", {
                disabled: !this.isValid,
                onclick: () => this.submit(),
            }, "Submit"),
        ]
    }
}

m.mount(document.querySelector("#form"), LunchtimePlayer)