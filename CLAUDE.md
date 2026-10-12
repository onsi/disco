# Disco — agent guide

Disco schedules pickup ultimate frisbee games for two Google Groups mailing lists. It is one Go
process on Fly.io (`ultimate-disco`) that serves `sedenverultimate.net`, receives inbound email as
webhooks from forwardemail.net, sends mail through Gmail SMTP, and runs two independent state
machines that invite players, count replies, and call game / no-game with the boss's approval.

## Layout

- `main.go` — wires everything. `ENV=PROD` gets real S3/outbox; anything else is DEV: fake outbox,
  fake in-memory DB seeded with sample snapshots (so the web pages have something to render), but
  the weather emoji cache still talks to real S3 under `DEV/`.
- `saturdaydisco/` — the Saturday game. Email-only: players reply to the list, an LLM interprets
  each reply into a head count (`interpreter.go`), the boss approves each outbound step by replying
  `/approve`, `/deny`, `/delay N`, etc. Templates are `templates/*.tmpl.md` (markdown → HTML).
- `lunchtimedisco/` — the weekday lunchtime game. Players pick game slots on a web page
  (`/lunchtime/:guid`); the boss drives it from a separate boss page (`/lunchtime/:bossguid`).
  Its spec suite is the only one that drives a browser (Biloba).
- `server/` — Echo routes, the `/subscribe` form (Cloudflare Turnstile), and the template renderer,
  which bundles `js/*.js` (Mithril, from `node_modules`) with esbuild **at request time** — cached in
  PROD, rebuilt every request in DEV. So `node_modules` must exist wherever the binary runs.
- `mail/` — `Email`/`EmailAddress` types, inbound parsing of forwardemail.net's JSON, `Outbox` (real
  SMTP) and `FakeOutbox`.
- `weather/` — api.weather.gov forecasts plus an LLM-picked emoji per short forecast, cached in S3.
- `askclaude/` — the LLM client: Claude Haiku (`claude-haiku-5-5`) via the Anthropic Go SDK, with
  structured outputs so callers always get schema-valid JSON. Haiku 5.5 rejects `temperature`; the
  schema is what pins the output shape.
- `clock/` — `AlarmClock` (one pending alarm, the state machines' only timer) and `FakeAlarmClock`;
  all scheduling is in `America/Denver`.
- `s3db/` — the persistence layer. Read the next section before touching any state.

## Persistence: S3 blobs + exactly one writer

There is no database. `s3db.S3DBInt` is two calls — `FetchObject(key)` and `PutObject(key, data)` —
over whole JSON blobs in one S3 bucket, with every key prefixed by `ENV` (`PROD/`, `DEV/`, `TEST/`).
There are no locks, no versions, no conditional writes: **last writer wins**. That is safe only
because **exactly one process ever writes a given `ENV` prefix**, and the design leans on it
everywhere. The invariants:

- **One Fly machine, always.** `fly.toml` pins `min_machines_running = 1` with auto start/stop off.
  Never `fly scale count` above 1, never add a region, never switch the deploy strategy to
  `bluegreen`/`canary` (both boot the new machine while the old one still runs, giving two writers
  and two alarm clocks sending duplicate emails). `fly.toml` pins `strategy = "rolling"`, which
  replaces the single machine in place, so the old process stops before the new one loads state.
- **Each disco is an actor.** `SaturdayDisco`/`LunchtimeDisco` embed their snapshot and own it from a
  single goroutine (`dance()`); everything else talks to it through channels (`commandC`,
  `snapshotC`, `templateC`). After every alarm and every command the loop marshals the *entire*
  snapshot and `PutObject`s it. Never read or mutate snapshot fields outside that loop, and never
  add a second code path that writes a disco's key.
- **Boot is the only read.** On startup each disco fetches its key: not found → fresh `reset()`;
  stale (from a previous week) → `reset()`; good → resume and re-arm the alarm at `NextEvent`
  (an overdue alarm fires right away). A fetch *error* or an *unmarshal error* emails the boss a
  `startup_error` and **refuses to start** — the process exits. Every boot also emails the boss a
  `startup` message, so every deploy sends one.
- **Snapshot schema changes must read old blobs.** The PROD blob was written by the previous
  build. Renaming/retyping a JSON field, or adding one whose zero value means something wrong,
  will either brick boot (unmarshal failure) or silently corrupt the week. Add fields so that the
  zero value is the correct default.
- **A failed backup is only logged.** In-memory state carries on; a crash before the next
  successful `PutObject` loses those changes.
- **Keys in use:** `saturday-disco`, `lunchtime-disco`, `lunchtime-participants` (the lunchtime
  address book, written only after `CommandSetGames`), `EmojiCache` (forecast → emoji;
  read-modify-write under the provider's mutex), and `email/<uuid>` (every raw inbound email, written
  fire-and-forget for debugging, never read by the code).
- **Tests use `s3db.FakeS3DB`** (in-memory, with `SetFetchError` to simulate S3 failures),
  except the `s3db` suite itself, which round-trips against the real bucket under `TEST/`.

## Config and secrets

`config.LoadConfig()` reads env vars. Locally the `makefile` `include`s `local.env` and `.secrets`
(gitignored, owner's laptop only) and exports them; on Fly the non-secret values are `[env]` in
`fly.toml` and the rest are `fly secrets`. A new secret has to be added in three places: `.secrets`,
`config.go`, and Fly. Use `fly secrets set --stage …` so it lands with the next deploy instead of
restarting the machine (and emailing the boss) on its own.

## House rules

- **✂️ NEVER COMMENT UNLESS NECESSARY — the default is NO comment.** Write a comment only to
  capture a **decision**, a **subtle interaction**, or a **genuine surprise that cannot be inferred
  from the code**. The code and its names document the code; the burden of proof is on the comment,
  never on its absence. If you cannot name which of the three a comment is, don't write it. When one
  clears the bar, state it in a sentence or two and stop.
  - **A comment is never a memoir.** No dates, no "used to", no "we tried X then Y", no account of
    the bug that motivated the line. `git log` owns history.
  - **Restate the fact; never point at a name.** A comment naming another symbol or file rots
    silently when that thing is renamed.
  - **Tell a fact once per file**, at the place it bites.
  - Nothing lints this here — review comments by eye on every diff.
- **Ginkgo for all Go tests — never `go test`.** Use the `make` targets, which load `local.env` and
  `.secrets` (the `s3db` suite needs the AWS credentials) and run `go run github.com/onsi/ginkgo/v2/ginkgo -r -p --keep-going --randomize-all`:
  `make test` (the whole suite), `make race` (the same under `--race`), and `make expensive` (adds
  the real-Haiku specs, gated on `INCLUDE_LLM_SPECS=true`; they spend tokens and are
  nondeterministic, so a prompt change needs several clean runs, not one).
- **Never pin a test server to a fixed port.** The lunchtime suite serves each process from an
  OS-assigned port it holds for the whole run, so concurrent runs (another worktree, an agent) can't
  steal each other's ports.
- **Keep the LLM out of the loop in ordinary specs.** The state machines take an `InterpreterInt`;
  their specs use `FakeInterpreter`. Only the interpreter's own spec and the emoji spec call the real
  model, behind the opt-in env var — those are the integration tests proving the prompt works.
- **Read the `biloba-go:*` skills BEFORE touching a Biloba spec (`lunchtimedisco/`). Every time.** At
  minimum `biloba-go:write-tests` and `biloba-go:flaky-specs`; the `ginkgo:*`/`gomega:*` skills are
  load-bearing the same way. Reading the existing specs is not a substitute. Upshot of the skills:
  - **Biloba polls by default.** Write the fully-applied form — `b.Click(sel)`, `b.SetValue(sel, v)`,
    `text := b.GetInnerText(sel)` — which polls until ready, acts/reads once, and stops. Keep the
    matcher form (`Eventually(sel).Should(b.Click())`) only when you own the poll (a `Consistently`,
    a composition, a non-default timeout). Never `b.Immediate()`; tune with `b.WithTimeout(d)`.
  - **Assert outcomes with `Eventually`; `Expect`/`Ω` only for values that cannot change.** Never
    assert on a value you read exactly once (`b.Run(expr, &x)` then `Ω(x)`) — poll it, or use
    `.Capture(&x)` on the matcher instead of gate-then-re-read.
  - **Watch for assertions that cannot fail**: a negated assertion on something that never rendered,
    an empty `Current*ForEach` snapshot under a negation. Anchor the subject first.
- **Flakes are always root-caused and fixed** — no timeout-widening, no re-run-until-green. A green
  streak is not proof a low-rate flake is dead.
- **Clean, modular architecture.** See duplication or cruft? Say so, record it in `TODO.md`, and ask
  whether to fix it now or later. Don't let it slide.

## Deploying

`make deploy` runs the suite, checks it compiles, then `flyctl deploy` (remote builder, `Dockerfile`).
The deploy restarts the one machine: the discos reload from S3 and the boss gets startup emails.
Fly health-checks `GET /healthz`, which deliberately touches nothing; never point a check at `/`, which
renders through both discos and calls the weather API. Avoid deploying in the windows where an alarm is about to fire for a list email (Tuesday 6am,
Thursday 2pm, Saturday morning for Saturday Disco); a boot after Thursday 2pm with the invite not yet
sent aborts the week and hands it to the boss.
