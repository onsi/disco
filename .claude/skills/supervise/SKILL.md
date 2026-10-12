---
name: supervise
description: Perform multiple tasks by spawning and supervising agents
---

You are a supervisor responsible for shepherding agents through Disco's backlog. Backlog items
usually live in `TODO.md` (gitignored), but follow the user's lead.

Preserve your context by having subagents do the work. Orient yourself on the codebase anyway —
especially the persistence section of `CLAUDE.md` — deeply enough to judge their work. You are
responsible for quality and correctness, and you curate the architecture: resist duplication, and
send a subagent to refactor when you see it.

## Picking a model and effort level

Dispatch developer work through one of the four profiles in `.claude/agents/`, which pin both
model and effort: `opus-dev-high`, `opus-dev-medium`, `sonnet-dev-high`, `sonnet-dev-medium`.
Prefer these to a bare `general-purpose` agent with a `model` override, which cannot set effort.

**Prefer the cheapest viable profile.** The test is not task size — it is whether the answer is
already specified, and how much load-bearing thinking remains on top of it.

- **`sonnet-dev-medium`** — mechanical, unambiguous work: a rename across known sites, a one-file
  fix the brief already names.
- **`sonnet-dev-high`** — the default. The brief contains the answer (a decision already made, a
  defect with a named fix, a mechanical refactor) but the task has moving parts: several files, a
  multi-step change, tests to reconcile.
- **`opus-dev-medium`** — Opus's judgment on a small point: an underspecified edge, a small design
  call, prompt wording that has to work on a real model.
- **`opus-dev-high`** — when the valuable output is something the brief could not ask for:
  open-ended investigation, a design with real trade-offs.

Writing a precise brief is how you buy the cheaper profile: state the decisions, name the files,
and say what would prove it works. Escalate when the cheap profile struggles — send the retry,
with your diagnosis pasted in, one rung up — but don't pre-emptively upgrade. Tell the user which
profile you picked for each workstream.

## Put comment etiquette in every brief, and enforce it on the diff

The comment rule in `CLAUDE.md` is the one subagents drift from most reliably: having just
reasoned hard, they leave the reasoning behind as a memoir. Restate it in every brief:

- the default is NO comment; the burden of proof is on the comment;
- one is warranted only for a **decision**, a **subtle interaction**, or a **genuine surprise that
  cannot be inferred from the code** — if the agent can't name which, it deletes it;
- it states the **fact**, never the history: no dates, no "used to", no account of the bug.

Nothing lints comments in this repo, so **read the diff yourself before the commit lands.** Count
comment lines added against product lines added before reading a word — a lopsided ratio is enough
to send it back. Watch for a new comment that rebuts an old one instead of replacing it, and for a
comment that points at another file or symbol instead of restating the fact. **Say what to KEEP**,
not just what to cut, so genuine surprises survive. Correct it via `SendMessage` to the agent that
wrote it, before the commit — it still holds the context.

## Scope the gates, and give every gate a timeout

Put the gate list in the brief, sized to the change:

- **Go** — `make test` (the whole suite, ~20s; it needs `.secrets` for the `s3db` suite).
- **Prompt or LLM client** — also `make expensive`, which runs the real-model specs. They spend real
  tokens and are nondeterministic, so a prompt change needs several clean runs, not one.
- **Docker / Fly** — `docker build` locally if Docker is available, and `fly deploy --build-only`
  to exercise the remote builder without releasing.
- **Biloba (`lunchtimedisco/`)** — `make test` covers it; for tight iteration
  `go run github.com/onsi/ginkgo/v2/ginkgo -p --randomize-all ./lunchtimedisco`.

**Pass an explicit `timeout` on every gate command and say so in the brief** —
`Bash(command: "make test", timeout: 600000)`. The default is 120s. An agent that trips it once tends
to background every later command, and **a backgrounded command does not wake a subagent that has
ended its turn** — it stalls forever. If you catch one stalled, read the output it's waiting on and
hand it the verdict.

## One writer at a time

A test run and a tree-editing agent cannot safely share a tree; neither can two agents both
editing `go.mod`. Serialize work in the main tree, or give a parallel workstream its own worktree
(`isolation: "worktree"`) and merge it afterwards. If a scope change lands mid-flight, send it to
the agent that already owns those files rather than starting a second writer.

## A green suite is not coverage

The closing pass on any change is: per behavioural change, what spec now proves it? **A regression
spec must FAIL against the old code** — require the subagent to say it checked. For a migration
(a library upgrade, a client swap) the bar is **no lost coverage**: compare spec counts and assertion
intent before and after, and treat a deleted or weakened assertion as a defect unless the agent
justifies it.

**Re-run the suites yourself** rather than trusting "tests pass", and check the claim covers the
current tree.

**You own flake hunts; subagents never run them.** After the Biloba suite changes, hunt it:
`go run github.com/onsi/ginkgo/v2/ginkgo -p --randomize-all --repeat=20 ./lunchtimedisco`. When a hunt
turns up a flake, reason from the failure output — don't try to reproduce it — and send it back to
the agent whose work introduced it, with your mechanism guess and explicit permission to disagree
with evidence. A pre-existing or cross-cutting flake goes to a *different* agent.

Smells found during a session are in scope for that session: record them in `TODO.md`, then
dispatch a subagent at them or consciously defer them with the user.

## Deploys are the user's call

Deploying restarts the one Fly machine, emails the boss, and reloads PROD state from S3 — read the
persistence and deploy sections of `CLAUDE.md`. Never deploy, scale, or change Fly secrets without
the user's go-ahead for that specific action.

Commit to master incrementally. Use branches only for riskier work or when the user asks.

At the end of a session, ask the user whether they'd like an audit of `CLAUDE.md` and the skills
for contradiction, duplication, verbosity, and incident-specific detail where a general rule
belongs.
