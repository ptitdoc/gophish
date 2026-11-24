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
  "Update workflow actions and Go versions (#3245)"). It holds the patch series below and
  nothing else.
- Previous branch: `postedit` holds the first three patches of the series, based on an
  older upstream. `postedit-202410` replays them on the 2024 upstream, then adds four more.
- The series is exported as `0001-*.patch` .. `0007-*.patch` at the repo root
  (`git format-patch master..postedit-202410`), to replay it on a later upstream with
  `git am`. The `*.bak` files at the root of their directories are leftovers from applying
  the patches by hand.
- `gophish-202401` at the root is a built binary (ELF x86-64), not source.

## Patch series (branch `postedit-202410`, oldest first)
| Commit | Date | Subject | What it changes |
|---|---|---|---|
| f804db4 | 2023-01-02 | hide banners | Comments out the `X-Server` header on the phishing server (`controllers/phish.go`) and the `X-Mailer` header on sent and test mails (`models/maillog.go`, `models/email_request.go`). |
| 65a47a5 | 2024-10-11 | change test email body | Replaces the default test mail subject and body with neutral text (`controllers/api/util.go`). |
| daa71e3 | 2024-10-11 | change rid to token | Renames the recipient URL parameter from `rid` to `token` (`models/campaign.go`, `RecipientParameter`). |
| 5d45e96 | 2025-01-14 | reference local gomail to add xoauth2 support | `go.mod` replaces `github.com/gophish/gomail` with the local path `/srv/gophish/gomail/`, a gomail copy with XOAUTH2 support; removes the gomail lines from `go.sum`. |
| 5cebc48 | 2025-01-14 | support custom throttling of email | Adds a send-rate selector (`#select_send_rate`) and a custom-rate modal (`#selectCustomRateModal`) to the campaign form (`templates/campaigns.html`). |
| 1cc1c3f | 2025-01-14 | custom throttling for email sent time | JS for the selector (`static/js/src/app/campaigns.js`): computes the upstream "send emails by" date from the rate and the group sizes, or the rate from that date. |
| 2aff248 | 2025-01-14 | hide hostname in smtp client | The SMTP client announces `localhost` in EHLO instead of the machine hostname (`models/smtp.go`). |
| -- | -- | QR code template functions | Templates can embed a QR code of the recipient link (`doc/FEATURE-qrcode.md`); `go.mod` replaces gomail with `../gomail`. |

## Conventions and known debts
- **Throttling is front-end only.** The rate is never sent to the API: the JS turns it into
  `send_by_date`, and the upstream backend spreads the mails up to that date. Reasoning:
  reusing the upstream mechanism keeps the backend untouched by the series.
- **Minified JS is not committed on this fork.** Upstream commits `static/js/dist/` so
  `go build` works without Node; this fork treats `static/js/dist/` as a build output and
  commits only `static/js/src/`. Reasoning: a generated file in git has to be regenerated
  and re-committed at every source change, and the patch series stays limited to source.
  Consequence: the committed `*.min.js` are upstream's, without the fork's JS changes (1cc1c3f
  has no `campaigns.min.js` counterpart). Rebuild them with upstream's gulp build before
  deploying; the working tree held a rebuilt set on 2026-11-23.
- **The gomail replacement is a sibling directory.** `go.mod` replaces
  `github.com/gophish/gomail` with `../gomail`, resolved from the repository root, so the
  gomail copy with XOAUTH2 support sits next to the gophish checkout. That copy is not in
  this repository: a clone sits in `~/gomail/` (upstream `gophish/gomail` plus one local
  commit, 1b86f64 "add support for smtp/XOAUTH2", on top of upstream 1f6d0df; checked
  2026-11-23). Reasoning: a relative path builds on any machine with that layout; the
  absolute `/srv/gophish/gomail/` of 5d45e96 only built where that path existed.
- **`rid` -> `token` breaks old links.** Mails sent before daa71e3 carry `?rid=`, which the
  patched server no longer reads.
- Fingerprint removals comment the upstream line out instead of deleting it. Reasoning: the
  original line stays visible next to the patch, which eases conflict resolution on rebase.

## Glossary
| Literature term | Code term | Plain term |
|---|---|---|
| Upstream (open-source fork terminology) | remote `github`, branch `master` | le projet gophish d'origine (FR) / the original gophish project (EN) |
| Patch series (git format-patch) | `0001-*.patch` .. `0007-*.patch`, branch `postedit-202410` | nos modifications, rejouables sur une nouvelle version (FR) / our changes, replayable on a new release (EN) |
| Send rate / throttling | `#select_send_rate`, `rate_update()` in `campaigns.js` | envoyer un mail toutes les N secondes (FR) / send one mail every N seconds (EN) |
| Recipient identifier | `RecipientParameter` (`token`, upstream `rid`) | le paramètre d'URL qui identifie la cible (FR) / the URL parameter that identifies the target (EN) |
| QR code (ISO/IEC 18004) | template function `qrcode`, `templateQRCode` | l'image carrée que le téléphone scanne pour ouvrir le lien (FR) / the square image a phone scans to open the link (EN) |
| Data URI (RFC 2397) | `data:image/png;base64,...` in an `<img src>` | l'image écrite directement dans le mail, sans fichier joint ni serveur (FR) / the image written inside the mail itself, with no attachment and no server (EN) |
