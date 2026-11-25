# Daily analytics design

## Intent

The blog owner wants daily unique-visitor counts. He builds the dashboard in
VictoriaLogs and does all deduplication there. Per-visitor telemetry stays
minimal, and visitor histories must be unrecoverable.

The field-level contract is [`docs/analytics-contract.md`](../../analytics-contract.md).

## Goals

- One stdout JSON line per successful home or post view by a non-bot agent.
- A daily visitor ID that cannot be reversed after the day ends and cannot be
  linked across days.
- No visitor state in the server: no counters, no seen-sets, nothing to lose on
  restart.
- Standard library only.

## Non-goals

- Exact unique counts across restarts. A mid-day restart overcounts.
- A dashboard, retention policy, or sink. The platform owns those.
- Tracking time on page, referrers, or navigation paths.

## Design

`internal/analytics.Recorder` holds a 32-byte random key for the current UTC
day, in memory only. The first request on a new UTC day overwrites it with fresh
`crypto/rand` bytes under a mutex. No goroutine runs. Each view prints
`visitor.id = HMAC-SHA256(day_key, ip + "\x00" + user_agent)[:32 hex]` and a
random UUIDv4 `event.id`. The router calls the recorder after `ListPosts` or
`GetPost` succeeds, so `4xx` and `5xx` responses print nothing.

## Rejected alternatives

| Alternative | Why rejected |
|---|---|
| In-memory dedup (print only the first view per visitor per day) | A restart, a memory cap, or a bug silently drops views that can never be recovered. The database can dedup; it cannot restore a line that was never printed. |
| Long-lived secret (`ANALYTICS_SALT_SECRET`) with a per-day derived key | Anyone holding the secret can recompute any past day's key and test candidate IPs against old IDs. A key that is discarded at the end of the day makes old IDs irreversible. Stable IDs across restarts are not worth that. |
| Cookie or `localStorage` ID | Stores an identifier on the visitor's device, needs a consent banner, and links visits across days. |
| Per-page visitor ID | Blocks site-wide daily uniques. Per-page uniques already come from grouping a site-wide ID by `page.slug`. |
| Printing the raw `User-Agent` | It is high-entropy and, joined with the time and page, can fingerprint a visitor. The HMAC input already separates browsers. |
