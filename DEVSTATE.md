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

## OPEN () [] -- tracker:dev:2611-001 -- 2026-11-24 -- Recipient parameter name set in config.json; fix TestNewTemplateContext
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

## NEXT () [] -- tracker:dev:2611-002 -- 2026-11-24 -- QR code template functions qrcode and base64
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
