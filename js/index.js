document.addEventListener('submit', (e) => {
    const form = e.target;
    let data = {
        email: document.querySelector("#email").value,
        wantsSaturday: document.querySelector("#saturday").checked,
        wantsLunchtime: document.querySelector("#lunchtime").checked,
        message: document.querySelector("#message").value,
    }
    fetch(form.action, {
        method: form.method,
        headers: {
            "Content-Type": "application/json",
        },
        body: JSON.stringify(data),
    }).then((res) => {
        if (!res.ok) throw new Error("Error subscribing");
        document.querySelector("#subscribe").outerHTML = "<div class='subscribe success'>Thanks for your interest!  We'll be in touch soon.</div>";
    }).catch((err) => {
        document.querySelector("#subscribe").outerHTML = "<div class='subscribe fail'>Sorry, there was an error getting you subscribed. Please try again later.</div>";
    })
    e.preventDefault();
});
