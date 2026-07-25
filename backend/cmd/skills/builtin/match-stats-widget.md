---
name: match-stats-widget
description: Football match scoreboard and stats widgets via TheSportsDB (score, timeline, lineups, scorers). Use when the user asks for match results, scores, stats, lineups, or scorers.
---

# match-stats-widget

Football match widgets: scoreboard + Stats / Timeline / Lineups / Scorers tabs, loaded live from TheSportsDB.

**Always create this widget** when the user asks for a match result, score, scoreline, who won, stats, lineup, scorers, or similar — do not answer with plain text alone.

**Data rules:** the widget must load everything at runtime from TheSportsDB via `fetch`. Do not scrape sites, do not use web search results, and do not hardcode scores, stats, scorers, lineups, or timelines into the HTML/JS. Set only CONFIG (`team1`, `team2`, optional `league`/`title`) from the user request; all match data comes from the API.

Output one self-contained HTML fragment (root `div` + IIFE `script`), inline styles only.

---

## Design

### Root

```html
<div id="match-stats-widget" style="width:100%;padding:0;margin:0;font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,Helvetica,Arial,sans-serif;background:var(--background);color:var(--foreground);">
```

- Full chat width; root padding/margin/border = 0
- Theme via CSS vars only: `--background`, `--foreground`, `--muted`, `--muted-foreground`, `--border`
- Never hardcode background/text colors — the host injects theme vars (light and dark)
- No max-width card, shadow, or `100vh` centering

### Layout (top → bottom)

1. **Header** — title left; status right (dot + label)
2. **Scoreboard** (centered) — home badge 64px + name | score `H - A` + status | away badge + name; muted subline `Venue · City · Date · Time`
3. **Tabs** (pills): Stats · Timeline · Lineups · Scorers  
   - Active: `background:var(--foreground);color:var(--background)`  
   - Inactive: transparent + `border:1px solid var(--border)`
4. **Content** — active tab body only

### Stats row

```
[home]   Label   [away]
[==== #22c55e ====][== #ef4444 ==]   track: var(--muted), h:6px, pill
```

- Drop: Total Shots, Blocked Shots, Shots insidebox / inside box, Shots outsidebox / outside box
- Put possession first when present (`Ball Possession`, `Possession`, …); show `%`
- If possession missing from API, skip it

### Timeline row

`min' | icon | player (± assist) | team · detail`  
Icons: Goal ⚽ · Yellow 🟨 · Red 🟥 · Sub ↔️ · VAR 📺 · else •

### Lineups

Two-column grid (`minmax(260px,1fr)`). Side = `strHome === 'Yes'` vs not (not `strTeam` — that is often the club). Number: `intSquadNumber`. Starters vs `strSubstitute === 'Yes'`.

### Scorers

Badge + player + muted team.

### Status colors

Loading `#f59e0b` · OK `#22c55e` · Error `#ef4444`

Escape dynamic strings used in `innerHTML`.

---

## CONFIG

```js
const CONFIG = {
  team1: '',
  team2: '',
  league: '',
  title: '',
  apiKey: '123'
};
const SDB = 'https://www.thesportsdb.com/api/v1/json/' + CONFIG.apiKey;
```

| Need | Set |
|------|-----|
| Match | `team1` + `team2` (required) |
| Filter | `league` — optional; omit or `''` when unknown |
| Custom header | `title` — optional |

`team1` / `team2` order does not matter. Real home/away come from the event. Use TheSportsDB team names (`Bayern Munich`, `Manchester City`, `Spain`, …).

**Latest match is automatic:** with `team1` + `team2`, TheSportsDB search returns the most recent meeting — no date or season needed. Add `league` only when the user names a competition or you need to disambiguate.

---

## TheSportsDB API

Base: `SDB + path`. JSON. Browser `fetch` with `{ mode: 'cors', credentials: 'omit' }`.

| Need | Path | Key |
|------|------|-----|
| Search | `/searchevents.php?e=` | `event` |
| Full event | `/lookupevent.php?id=` | `events[0]` |
| Stats | `/lookupeventstats.php?id=` | `eventstats` |
| Timeline | `/lookuptimeline.php?id=` | `timeline` |
| Lineup | `/lookuplineup.php?id=` | `lineup` |

### Event fields

`idEvent`, `strEvent`, `strLeague`, `strHomeTeam`, `strAwayTeam`, `strHomeTeamBadge`, `strAwayTeamBadge`, `intHomeScore`, `intAwayScore`, `strStatus` (prefer over `strProgress`), `strTimestamp`, `dateEvent`, `strTime`, `strVenue`, `strCity`, `strResult`, `strDescriptionEN`

### Stats item

`strStat`, `intHome`, `intAway`

### Timeline item

`strTimeline`, `strTimelineDetail`, `strPlayer`, `strAssist`, `strTeam`, `strHome`, `intTime`, `strComment` (ignore `"NULL"`)

### Lineup item

`strPlayer`, `strPosition`, `intSquadNumber`, `strHome`, `strSubstitute`, `strTeam` (club; not for side split)

---

## Resolve match

```
team1 + team2 required
queries (both orders):
  Team1_vs_Team2 , Team2_vs_Team1
  "Team1 vs Team2" , "Team2 vs Team1"
slug: spaces → _
for each query: searchevents
keep events where both team names match (either side)
rank: league hit, has score, newer dateEvent
lookupevent(winner.idEvent) to hydrate full record
```

This always resolves to the **latest** match between the two teams (optionally narrowed by `league`). After resolve, stats/timeline/lineup use that event’s `idEvent`.

---

## Load

```
match = resolveMatch()
render scoreboard
Promise.all stats + timeline + lineup (.catch → [])
normalize stats
scorers = timeline goals; else light parse of strResult / strDescriptionEN
status OK → render tab
```

Badges: API badge URLs first; else `https://flagcdn.com/w80/{cc}.png` via `<img>`.

---

## Skeleton

```js
(function () {
  const CONFIG = { team1: '', team2: '', league: '', title: '', apiKey: '123' };
  const SDB = 'https://www.thesportsdb.com/api/v1/json/' + CONFIG.apiKey;
  const state = { match: null, stats: [], timeline: [], lineup: [], scorers: [], tab: 'stats' };

  async function sdbJson(path) {
    const res = await fetch(SDB + path, { mode: 'cors', credentials: 'omit' });
    if (!res.ok) throw new Error('HTTP ' + res.status);
    return res.json();
  }

  // helpers: esc, resolveMatch/searchPair, renders, tabs, load
  load();
})();
```

Implement helpers fully. Tab click restyles pills and re-renders. Empty sections: muted one-liner. Failures: red status + message.

---

## CONFIG examples

```js
{ team1: 'Spain', team2: 'Argentina', league: '', title: '', apiKey: '123' }

{ team1: 'Real Madrid', team2: 'Manchester City', league: 'UEFA Champions League', title: '', apiKey: '123' }

{ team1: 'Bayern Munich', team2: 'Inter Milan', league: '', title: '', apiKey: '123' }
```
