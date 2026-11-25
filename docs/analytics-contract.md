# Blog analytics contract

`blog-web` prints one JSON line per page view so the owner can count daily
unique visitors in VictoriaLogs. The server keeps no visitor state. It prints
every qualifying view, and all deduplication happens in the database.

## Stream

| Item | Value |
|---|---|
| `event.name` | `blog.analytics` |
| `event.version` | `1` |
| Stream | stdout of the `blog-web` container. Diagnostics and access logs stay on stderr. |
| Path | Fluent Bit to VictoriaLogs, 30-day retention. |
| Envelope | `nacfson_pipeline/contracts/CONTRACT_LOG_SCHEMA.md` §2. |

## Which views are printed

A line is printed when one of these responses succeeds with `200`:

| Request | `page.kind` |
|---|---|
| `GET /api/posts` | `home` |
| `GET /api/posts/{slug}` | `post` |

No line is printed for `4xx` or `5xx` responses, for other routes, or for
excluded agents. An agent is excluded when its `User-Agent` is empty or matches
`bot|crawl|spider|slurp|facebookexternalhit|preview|monitor|curl|wget|python-requests|go-http-client|headless|kube-probe`
(case-insensitive).

## Fields

Each line has exactly these keys.

| Key | Example | Meaning |
|---|---|---|
| `time` | `2026-11-25T12:00:00.123Z` | Print time, UTC, milliseconds. |
| `level` | `info` | Always `info`. |
| `msg` | `page view` | Always `page view`. |
| `service.name` | `blog-web` | Contract §2. |
| `service.version` | `v1.4.2` | Contract §2. |
| `event.name` | `blog.analytics` | Event category. |
| `event.version` | `1` | Schema version of this table. |
| `event.id` | `0b6f1c9e-3d2a-4f5b-9c7d-1e2f3a4b5c6d` | Random UUIDv4, unique per printed line. |
| `analytics.day` | `2026-11-25` | UTC day the view was counted in. |
| `visitor.id` | `9f2c4e1a7b3d5f60a1b2c3d4e5f60718` | 32 hex chars. Same visitor, same UTC day, same process: same value. |
| `page.kind` | `home` or `post` | Viewed page. |
| `page.slug` | `the-first-footprint` | Post lines only. |
| `page.topic` | `cloud-native` | Post lines only. |

Lines never contain the client IP, the `User-Agent`, the referrer, the query
string, cookies, or `trace_id`. `trace_id` is left out so a view cannot be
joined to the access log, which does carry the path and trace.

## Privacy model

- No cookies or browser storage. The visitor is derived from the request.
- `visitor.id` is the first 32 hex chars of
  `HMAC-SHA256(day_key, client_ip + "\x00" + user_agent)`.
  `client_ip` is `X-Real-Ip` (set by Traefik), or the socket peer when absent.
- `day_key` is 32 random bytes from `crypto/rand`. It exists only in process
  memory. It is never printed, stored, or exported. The first request of a new
  UTC day overwrites it with fresh random bytes.
- After the day ends the key is gone, so nobody, including the owner, can map a
  `visitor.id` back to an IP and `User-Agent`.
- IDs for the same visitor on different days are unrelated, so visitor
  histories across days cannot be rebuilt.
- `visitor.id` is site-wide for the day, not per page. Per-page uniques come
  from grouping by `page.slug`.

## Deduplication in the database

The server never skips a view as a repeat. Two kinds of duplicates reach the
database:

| Duplicate | Cause | Remove with |
|---|---|---|
| Delivery replay | Fluent Bit retries resend a line. | `count_uniq(event.id)` |
| Repeat visit | The same visitor views again that day. | `count_uniq(visitor.id)` |

## Known overcount

A process restart mid-day, including a rolling deploy, draws a new key. A visitor
seen before and after the restart gets two `visitor.id` values that day. This
overcounts uniques by at most one ID per visitor per restart. No view is lost.
The deployment runs one replica; each extra replica would hold its own key and
add the same kind of overcount.

## Example LogsQL queries

Group by `analytics.day`, not `_time:1d`. `visitor.id` is scoped to
`analytics.day`, and `_time` is the ingestion time, which can fall on the next
day for a view printed just before UTC midnight.

Daily unique visitors:

```logsql
_time:30d event.name:="blog.analytics"
  | stats by (analytics.day) count_uniq(visitor.id) as visitors
  | sort by (analytics.day)
```

Daily page views, with delivery replays removed:

```logsql
_time:30d event.name:="blog.analytics"
  | stats by (analytics.day) count_uniq(event.id) as views
  | sort by (analytics.day)
```

Unique viewers per post per day:

```logsql
_time:30d event.name:="blog.analytics" page.kind:="post"
  | stats by (analytics.day, page.slug) count_uniq(visitor.id) as viewers
  | sort by (analytics.day, viewers desc)
```

Syntax follows the VictoriaLogs LogsQL docs for
[exact filters](https://docs.victoriametrics.com/victorialogs/logsql/#exact-filter),
the [`stats` pipe](https://docs.victoriametrics.com/victorialogs/logsql/#stats-pipe),
and [`count_uniq`](https://docs.victoriametrics.com/victorialogs/logsql/#count_uniq-stats).
