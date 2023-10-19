export class EmailAddress {
    static fromEmail(email) {
        return new EmailAddress(email)
    }

    static fromNameAndAddress(name, address) {
        if (name.trim().length == 0) {
            return new EmailAddress(address.trim())
        }
        return new EmailAddress(`${name.trim()} <${address.trim()}>`)
    }

    static isValidAddress(address) {
        return address.includes("@") && address.includes(".")
    }

    constructor(email) {
        this.email = email
    }

    get isValid() {
        return EmailAddress.isValidAddress(this.address)
    }

    get hasExplicitName() {
        let tidy = this.string
        return tidy.lastIndexOf(" ") != -1
    }

    get string() {
        return this.email.trim()
    }

    get name() {
        let tidy = this.string
        if (tidy.lastIndexOf(" ") == -1) {
            return this.address.split("@")[0]
        }
        return tidy.split(" ")[0].trim()
    }

    get fullName() {
        let tidy = this.string
        if (tidy.lastIndexOf(" ") == -1) {
            return this.address.split("@")[0]
        }
        return tidy.split(" ").slice(0, -1).join(" ").trim()
    }

    get address() {
        let tidy = this.string
        if (tidy.lastIndexOf(" ") == -1) {
            return tidy
        }
        return tidy.split(" ").slice(-1)[0].trim().slice(1, -1)
    }

    toJSON() {
        return this.string
    }

    equals(other) {
        return this.address == other.address
    }
}