# DEVSTATE -- gophish fork

## How to read this file
- Written by the projdocs `tracker_*` tools. A logbook: one entry per item, appended at the end, never moved or rewritten.
- What the file tracks, and why: the `## Summary` section, right before `## Log`. Its last
  `### Revision <n> -- <date>` is the current one; without any, the whole section is.
- Closed tags: DONE the work landed; SUPERSEDED replaced by the ref in its `()`;
  CANCELLED dropped, the reason on its `Judgement: cancelled` line.
- Open items: `grep -n '^## \(NEXT\|OPEN\) ' DEVSTATE.md`. The last `NEXT` is the current
  one; read that entry only.
- Entries referencing an item: `grep -n -- 'tracker:<tracker>:<id>' DEVSTATE.md` -- the item's own heading, and every entry whose () holds it.

## Summary
Development work on the gophish fork: new patches of the series (fingerprint removals and operational features upstream does not have) and fixes to existing ones. Each item that lands becomes one commit of the series on branch `postedit-202410`.

## Log

## NEXT () [H] -- tracker:dev:2611-001 -- 2026-11-24 -- Recipient parameter name set in config.json; fix TestNewTemplateContext
Context: daa71e3 "change rid to token" hard-codes the recipient URL parameter as `token` in `models/campaign.go` (`RecipientParameter`). `models/template_context_test.go` (`TestNewTemplateContext`) still expects `?rid=` in `URL` and `TrackingURL`, so it most likely fails since daa71e3 (read, not run, 2026-11-24).

Goal: the parameter name comes from the configuration, so each assessment picks its own name without a rebuild.

Plan:
- Read the name from `config.json` at startup and set `RecipientParameter` from it.
- Adapt `TestNewTemplateContext` to build the expected URLs from `RecipientParameter` instead of a hard-coded `rid`.
- Add a test that a configured name ends up in `URL` and `TrackingURL`.
- Update the CLAUDE.md patch series table and the `rid -> token` known debt.

Defaults if nobody decides:
- Config key `recipient_parameter` at the top level of `config.json`. Reasoning: the parameter is used by both the phishing server (reading clicks) and the mail generation, so it belongs to neither `admin_server` nor `phish_server`.
- Key absent: `token`, the value the fork already uses. Reasoning: an existing `config.json` keeps working unchanged.

Acceptance criteria:
- `go test ./models/` green, `TestNewTemplateContext` included.
- With `recipient_parameter` set to another name, a test mail carries `?<name>=` and a click on it is recorded.
Handover: 2026-11-25 agent=main session model=claude-opus-5-5 cost=none reason=Implementation done in the working tree, waiting for the user's commit and live test. status=Implemented, uncommitted; models suite at known failures only; live test not done.
Handover: 2026-11-25 agent=main session model=claude-opus-5-5 cost=none reason=User committed be6e9bb; the CLAUDE.md patch-series table was removed at the user's request, which voids the previous Next. status=Committed (be6e9bb); live test pending.

## DONE (commit:68c6ef6) [H] -- tracker:dev:2611-002 -- 2026-11-24 -- QR code template functions qrcode and base64
Context: assessments need a QR code in phishing mails that carries the recipient URL. A remote QR service works today (`<img src="https://api.qrserver.com/v1/create-qr-code/?size=250x250&data={{.URL | urlquery}}">`, checked 2026-11-24), but it sends every recipient token to a third party and is a remote image that clients block or proxy.

Goal (variant 1): gophish generates the QR code itself and the template embeds it as a data URI:
`<img src="data:image/png;base64,{{.URL | qrcode | base64}}">`

Plan:
- Dependency `github.com/skip2/go-qrcode` (pure Go; not yet in go.mod nor in the module cache on 2026-11-24, needs a download).
- `models/template_context.go`: `ExecuteTemplate` registers a `template.FuncMap` with `qrcode` (string -> PNG bytes, 256 px, medium error correction) and `base64` (bytes or string -> standard base64). Reasoning: every phishing content (mail subject/text/HTML/headers, landing page, redirect URL, attachments, template validation) goes through `models.ExecuteTemplate` (grep 2026-11-24, `controllers/route.go` only renders the admin UI), so one registration covers them all. Functions instead of a `.QRCode` field: the PNG is computed only when a template uses it, and any URL can be encoded (`.TrackingURL`, ...).
- Lowercase names, like the built-in `urlquery`.
- Tests in `models/template_context_test.go`, named `TestFeat_2611_002_*`.
- CLAUDE.md: patch series row, glossary row.

Defaults if nobody decides: 256 px and medium error correction, fixed constants (no config key until a size is actually needed).

Out of scope, later items: variant 2 (inline `cid:` image attachment, which mail clients display more reliably than a data URI).

Acceptance criteria:
- `{{.URL | qrcode | base64}}` renders a base64 string that decodes to a PNG whose QR content equals `.URL`, checked with a decoder that shares no code with the generator (`zbarimg` or a phone scan).
- `ValidateTemplate` accepts a template using both functions.
- `go test ./models/` green, apart from failures already present before the change (see tracker:dev:2611-001).
Handover: 2026-11-24 agent=main session model=claude-opus-5-5 cost=turn 41  ctx 100k  floor 50k (turn 1)  cumulated 3.1M tok  est. $2.16  model claude-opus-5-5 reason=Session cleared after commit 68c6ef6, before the live acceptance check. status=Implemented and committed (68c6ef6); entry stays NEXT until the live scan check passes.

## NEXT (tracker:dev:2611-002) [H] -- tracker:dev:2611-003 -- 2026-11-24 -- QR code as an inline cid: image attachment (variant 2)
Context: tracker:dev:2611-002 (commit 68c6ef6) added the template functions `qrcode` and `base64`, so a template embeds the QR code of the recipient link as a data URI: `<img src="data:image/png;base64,{{.URL | qrcode | base64}}">`. Mail clients display an inline `cid:` attachment more reliably than a data URI. This item was marked out of scope of 2611-002 ("variant 2"); the reference points to that origin.

Goal: the QR code of `.URL` reaches the recipient as an inline image attachment, referenced from the HTML as `<img src="cid:qrcode.png">`.

Default if nobody decides: an attached image named `qrcode.png` is replaced per recipient by the QR code of `.URL` in `Attachment.ApplyTemplate`, and the HTML uses `<img src="cid:qrcode.png">`. Reasoning: it reuses the existing inline embedding (`msg.Embed` in models/maillog.go) and needs no UI change. Not checked against the code yet (2026-11-24): read `Attachment.ApplyTemplate` and the `msg.Embed` path before planning.

Constraints: reuse `templateFuncs` / the go-qrcode generator from 68c6ef6 (256 px, medium error correction); one patch of the series, small enough to replay on a newer upstream.

Acceptance criteria (draft, to confirm when planning):
- A campaign mail carries an inline `qrcode.png` part whose QR content equals the recipient `.URL`, checked with a decoder that shares no code with the generator (`zbarimg` or a phone scan).
- Each recipient gets their own QR code (two recipients, two different tokens).
- Tests `TestFeat_<id>_*` in `models/`; `go test ./models/` green apart from pre-existing failures (see tracker:dev:2611-001).
- `doc/FEATURE-qrcode.md` documents the variant and its reasoning.
Handover: 2026-11-24 agent=main session model=claude-opus-5-5 cost=none reason=Design settled with the user (2026-11-24); it replaces the default written in the entry body. status=Design chosen, code not read yet, nothing implemented.
Handover: 2026-11-24 agent=main session model=claude-opus-5-5 cost=turn 39  ctx 104k  floor 51k (turn 1)  cumulated 2.8M tok  est. $1.85  model claude-opus-5-5 reason=Implementation done in the working tree, waiting for the full suite run and the user's commit. status=Implemented, uncommitted; feature tests green, full models suite not run yet, live scan not done.
Handover: 2026-11-24 agent=main session model=claude-opus-5-5 cost=none reason=User committed 98d6a2c; entry stays NEXT until the live scan check passes. status=Committed (98d6a2c); live scan check pending.

## DONE (commit:f804db4) [] -- tracker:dev:2611-004 -- 2026-11-25 -- Hide banners: X-Server and X-Mailer headers
Patch of the series, landed before this tracker existed (2023-01-02). Comments out the `X-Server` header on the phishing server (`controllers/phish.go`) and the `X-Mailer` header on sent and test mails (`models/maillog.go`, `models/email_request.go`). Known consequence: `TestEmailRequestGenerate` still expects `X-Mailer: gophish` and fails.

## DONE (commit:65a47a5) [] -- tracker:dev:2611-005 -- 2026-11-25 -- Neutral test email subject and body
Patch of the series, landed before this tracker existed (2024-10-11). Replaces the default test mail subject and body with neutral text (`controllers/api/util.go`).

## DONE (commit:daa71e3) [] -- tracker:dev:2611-006 -- 2026-11-25 -- Recipient URL parameter renamed from rid to token
Patch of the series, landed before this tracker existed (2024-10-11). Renames the recipient URL parameter from `rid` to `token` (`models/campaign.go`, `RecipientParameter`). Made configurable later by tracker:dev:2611-001.

## DONE (commit:5d45e96) [] -- tracker:dev:2611-007 -- 2026-11-25 -- Local gomail copy with XOAUTH2 support
Patch of the series, landed before this tracker existed (2025-01-14). `go.mod` replaces `github.com/gophish/gomail` with the local path `/srv/gophish/gomail/`, a gomail copy with XOAUTH2 support; removes the gomail lines from `go.sum`. The path became `../gomail` in 68c6ef6 (tracker:dev:2611-002).

## DONE (commit:1cc1c3f) [] -- tracker:dev:2611-008 -- 2026-11-25 -- Custom email send rate (throttling)
Two patches of the series, landed before this tracker existed (2025-01-14):
- 5cebc48: send-rate selector (`#select_send_rate`) and custom-rate modal (`#selectCustomRateModal`) in the campaign form (`templates/campaigns.html`).
- 1cc1c3f: JS for the selector (`static/js/src/app/campaigns.js`): computes the upstream "send emails by" date from the rate and the group sizes, or the rate from that date.

## DONE (commit:2aff248) [] -- tracker:dev:2611-009 -- 2026-11-25 -- SMTP client announces localhost in EHLO
Patch of the series, landed before this tracker existed (2025-01-14). The SMTP client announces `localhost` in EHLO instead of the machine hostname (`models/smtp.go`).
