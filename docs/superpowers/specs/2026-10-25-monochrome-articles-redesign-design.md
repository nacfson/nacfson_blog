# Monochrome Articles Redesign (Make Better World)

Date: 2026-10-25 · Status: approved in chat ("I love monochrome" / "Ok. Update its real code too")

## Intent
Rebuild the homepage using the *structure* of https://claude.com/resources/articles, but in a
monochrome editorial palette (the user found the previous blue theme looked like "AI slop").
The blog was just emptied, so the empty state must look intentional and friendly.

## Decisions (user)
- Palette: **Monochrome Editorial**: ink `#0c0a09` (stone-950) on stone `#fafaf9` (stone-50),
  white cards, grey `#a8a29e` (stone-400), serif headlines, 4px radius, hatch-pattern plates, no accent hue.
- Layout: top nav → sub-nav ("Make Better World") → left rail (topics + about card) + main
  (hero, featured shelf, toolbar, card grid, View more) → light multi-column footer.
- Posts declare a `topic` (replaces `pillar`), limited to the four interests:
  `cloud-native`, `zero-trust`, `ai-agents`, `open-source`.
- Empty UI: a notification-style notice: "The blogger is writing the first story…".

## My calls (reviewable)
- h1 stays **"Make Better World"** (the earlier explicit requirement); the summary is the subtitle.
- Featured shelf = up to 4 newest posts; hidden when there are no posts.
- "View more" shows 9 cards at a time, client-side (all posts already fetched once).
- Topic filter is client-side and synced to `?topic=` so footer links work; API also accepts `?topic=`.
- Article reader keeps its layout; only colors move to monochrome (blocks included).
- Topic-filtered empty state: "Nothing in <topic> yet" + "Show all articles".
- `pillar` is removed entirely (Go, TS, fixtures); posts missing a valid `topic` fail validation.

## Non-goals
- No newsletter/email subscription (no backend for it); the notice links to GitHub instead.
- No dark mode.
