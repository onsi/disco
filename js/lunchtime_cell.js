import m from "mithril"

export function ClassForCount(count) {
    if (count >= 5) {
        return "quorum"
    } else if (count >= 3) {
        return "close"
    } else if (count >= 1) {
        return "barely"
    }
    return "zero"
}

export class LunchtimeCell {
    view(vnode) {
        let game = vnode.attrs.game
        let players = vnode.attrs.players
        let count = players.length
        let f = game.forecast
        return m("td.game",
            {
                id: game.key,
                class: ClassForCount(count) + (vnode.attrs.selected ? " selected" : ""),
                onclick: vnode.attrs.onclick,
            },
            m("div.time", game.time),
            m("div.count", `${count}`),
            count > 0 ? m("div.players", players.map(p => m(".player", p))) : null,
            f.shortForecast ? [
                m("div.forecast",
                    m("span.emoji", f.ShortForecastEmoji),
                ),
                m("div.forecast",
                    m("span.text", `${f.temperature}°${f.temperatureUnit}`),
                ),
                m("div.forecast-details", `💧${f.ProbabilityOfPrecipitation}%`),
                m("div.forecast-details", `💨 ${f.windSpeed}`),
            ] : null,
        )
    }
}