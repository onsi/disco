# Disco 🪩

Disco is a heavily over-engineered solution to the problem of scheduling ultimate frisbee games.  It provides:

- A web-page at sedenverultimate.net hosted on fly.io
- The ability to send invite and call game/no-game on these two mailing lists:
    - saturday-se-denver-ultimate@googlegroups.com
    - lunchtime-se-denver-ultimate@googlegroups.com
- A chatbot that monitors for chatter on the aforementioned mailing lists

## Third-Party Accounts/Things Needed to run Disco

All credentials are in a `.secrets` file on Onsi's laptop or stored securely in fly.io.  Disco depends on:

- fly.io for running the tiny Go server
- amazon Route 53 is the DNS registrar
- "database" is backed up on Amazon S3
- e-mail sneding and forwarding for the disco bot is handled by forwardemail.net
- language parsing is handled by openai.com