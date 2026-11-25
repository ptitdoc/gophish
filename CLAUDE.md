# CLAUDE.md -- gophish fork

## Project philosophy
This repository is a fork of gophish (https://github.com/gophish/gophish), used to run
phishing simulation campaigns during security assessments. The fork exists to keep a small,
readable set of patches on top of upstream: each patch removes a gophish fingerprint from
the mails and the phishing server, or adds an operational feature upstream does not have.
Every choice serves one idea: the patch series must stay small enough to be replayed on a
newer upstream release. Reasoning: upstream keeps moving, and a fork that diverges in many
places cannot be rebased any more.

## How the project was taken over
- Remotes: `github` is upstream (`gophish/gophish`), `origin` is the personal fork
  (`ptitdoc/gophish`). Local `master` tracks upstream `github/master`; `origin/master` is
  older (db63ee9, 2021-03-28) and is not the base of the current work.
- Working branch: `postedit-202410`, based on upstream `master` at 9561846 (2024-09-22,
  "Update workflow actions and Go versions (#3245)"). It holds the patch series and
  nothing else (`git log master..postedit-202410`).
- Previous branch: `postedit` holds the first three patches of the series, based on an
  older upstream. `postedit-202410` replays them on the 2024 upstream, then adds four more.
- The series is exported as `0001-*.patch` .. `0007-*.patch` at the repo root
  (`git format-patch master..postedit-202410`), to replay it on a later upstream with
  `git am`. The `*.bak` files at the root of their directories are leftovers from applying
  the patches by hand.
- `gophish-202401` at the root is a built binary (ELF x86-64), not source.

## What the fork changes
Fingerprint removals:
- The phishing server sends no `X-Server` header, and mails carry no `X-Mailer` header
  (`controllers/phish.go`, `models/maillog.go`, `models/email_request.go`).
- The SMTP client announces `localhost` in EHLO instead of the machine hostname
  (`models/smtp.go`).
- The default test mail has a neutral subject and body (`controllers/api/util.go`).
- The recipient URL parameter is `token` instead of upstream `rid`, or the name set by
  `recipient_parameter` in `config.json` (`RecipientParameter`, `models/campaign.go`).

Operational features:
- SMTP authentication with XOAUTH2, through the gomail copy `go.mod` points to (see
  "The gomail replacement is a sibling directory" below).
- A send-rate selector on the campaign form: one mail every N seconds, turned into the
  upstream "send emails by" date (`templates/campaigns.html`,
  `static/js/src/app/campaigns.js`; see "Throttling is front-end only" below).
- Template functions `qrcode`, `base64` and `attachment`: a mail embeds a QR code of the
  recipient link, as a data URI or as an inline `cid:` part (`doc/FEATURE-qrcode.md`).

## Build
What to rebuild depends on what changed:

| Changed | Action |
|---|---|
| Go code (`*.go`, `go.mod`) | `go build` |
| `static/js/src/`, `static/css/` | JS build, then deploy `static/` |
| `templates/*.html` | nothing to build: gophish reads them from disk on every page (`controllers/route.go`, `ParseFiles`); deploy the file |

**Go binary**, from the repository root:
`go build -o gophish . 2>&1 | tee /tmp/gophish-build.log`.
- `go.mod` replaces gomail with `../gomail`: the gomail copy must sit next to the gophish
  checkout, or the build fails on `reading ../gomail/go.mod`.
- `github.com/mattn/go-sqlite3` is compiled with cgo, so gcc must be installed. Its
  `-Wreturn-local-addr` warning in `sqlite3-binding.c` is upstream's and harmless.

**JS build** (`static/js/dist/`, `static/css/dist/`), from the repository root:
`npm install 2>&1 | tee /tmp/gophish-npm.log && npx gulp 2>&1 | tee /tmp/gophish-gulp.log`.
- `npx gulp scripts` rebuilds only `static/js/dist/app/*.min.js`; `npx gulp` also
  rebuilds `vendor.min.js` and the CSS (`gulpfile.js`).
- `gulp` and `gulp-cli` are devDependencies, so `npx` runs the local copy; no global
  install is needed (upstream's `Dockerfile` installs them globally).
- On Debian and Ubuntu, the `yarn` command from the `cmdtest` package is an unrelated test
  runner (`ERROR: [Errno 2] No such file or directory: 'install'`). Debian's real Yarn is
  `yarnpkg`. `npm install` ignores `yarn.lock`, so tool versions may differ from a Yarn
  build.
- The output is not committed (see "Minified JS is not committed" below).

**Tests**: no Makefile; `go test ./models/ > /tmp/gophish-test.txt 2>&1`.

## Conventions and known debts
- **Each patch is tracked in the `dev` tracker, not in this file.** A patch's entry holds
  its goal and is closed with its commit hash; `git log master..postedit-202410` lists the
  series. This file describes what the fork does ("What the fork changes"), with no
  commit, date or status. Reasoning: this file describes the code and its conventions as
  they are, and a per-commit list here duplicates the tracker and needs an edit after
  every commit.
- **Throttling is front-end only.** The rate is never sent to the API: the JS turns it into
  `send_by_date`, and the upstream backend spreads the mails up to that date. Reasoning:
  reusing the upstream mechanism keeps the backend untouched by the series.
- **Minified JS is not committed on this fork.** Upstream commits `static/js/dist/` so
  `go build` works without Node; this fork treats `static/js/dist/` as a build output and
  commits only `static/js/src/`. Reasoning: a generated file in git has to be regenerated
  and re-committed at every source change, and the patch series stays limited to source.
  Consequence: the committed `*.min.js` are upstream's, without the fork's JS changes (1cc1c3f
  has no `campaigns.min.js` counterpart). Rebuild them before deploying (see "Build"); the
  working tree held a rebuilt set on 2026-11-23.
- **The gomail replacement is a sibling directory.** `go.mod` replaces
  `github.com/gophish/gomail` with `../gomail`, resolved from the repository root, so the
  gomail copy with XOAUTH2 support sits next to the gophish checkout. That copy is not in
  this repository: a clone sits in `~/gomail/` (upstream `gophish/gomail` plus one local
  commit, 1b86f64 "add support for smtp/XOAUTH2", on top of upstream 1f6d0df; checked
  2026-11-23). Reasoning: a relative path builds on any machine with that layout; the
  absolute `/srv/gophish/gomail/` of 5d45e96 only built where that path existed.
- **Changing the recipient parameter name breaks old links.** The server reads only the
  name currently set: mails sent before daa71e3 carry `?rid=`, and mails sent before a
  change of `recipient_parameter` carry the old name. Keep the name for the whole
  campaign. The key sits at the top level of `config.json`. Reasoning: the phishing server
  reads it on clicks and the mail generation writes it, so it belongs to neither
  `admin_server` nor `phish_server`.
- Fingerprint removals comment the upstream line out instead of deleting it. Reasoning: the
  original line stays visible next to the patch, which eases conflict resolution on rebase.

## Glossary
| Literature term | Code term | Plain term |
|---|---|---|
| Upstream (open-source fork terminology) | remote `github`, branch `master` | le projet gophish d'origine (FR) / the original gophish project (EN) |
| Patch series (git format-patch) | `0001-*.patch` .. `0007-*.patch`, branch `postedit-202410` | nos modifications, rejouables sur une nouvelle version (FR) / our changes, replayable on a new release (EN) |
| Send rate / throttling | `#select_send_rate`, `rate_update()` in `campaigns.js` | envoyer un mail toutes les N secondes (FR) / send one mail every N seconds (EN) |
| Recipient identifier | `RecipientParameter`, config key `recipient_parameter` (default `token`, upstream `rid`) | le paramètre d'URL qui identifie la cible (FR) / the URL parameter that identifies the target (EN) |
| QR code (ISO/IEC 18004) | template function `qrcode`, `templateQRCode` | l'image carrée que le téléphone scanne pour ouvrir le lien (FR) / the square image a phone scans to open the link (EN) |
| Data URI (RFC 2397) | `data:image/png;base64,...` in an `<img src>` | l'image écrite directement dans le mail, sans fichier joint ni serveur (FR) / the image written inside the mail itself, with no attachment and no server (EN) |
| Inline body part referenced by a Content-ID URL (`cid:`, RFC 2392) | template function `attachment`, collector `templateAttachments`, `embedTemplateAttachments` | une pièce jointe générée pour chaque destinataire et affichée dans le corps du mail (FR) / a file generated for each recipient, attached and shown inside the mail body (EN) |
