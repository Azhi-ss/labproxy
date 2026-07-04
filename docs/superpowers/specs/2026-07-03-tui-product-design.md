# LabProxy TUI Product Design

- Date: 2026-07-03
- Status: proposal
- Scope: terminal UI/UX for `labproxy tui`
- Principle: CLI-first control plane, polished TUI as a human console

## 0. Source Of Truth

Use this document for the target TUI design. For the current shipped behavior,
inspect the installed binary at `~/.labproxy/bin/labproxy-tui` or the source
under `internal/tui/`.

Do not use stale screenshot assets or ignored `build/` binaries as design
references. Earlier resources showed an old three-tab `Clash TUI`; those do not
represent either the current shipped TUI or this target design.

## 1. Product Positioning

LabProxy TUI should feel like a terminal-native proxy operations console, not a
decorated command output. It is for checking whether the proxy is healthy,
understanding where traffic is going, switching nodes quickly, and diagnosing bad
routes without opening a browser dashboard.

The design target is closer to `lazygit`, `k9s`, and Clash Verge's information
model than to a generic table viewer.

## 2. Users And Jobs

Primary user: a technical user who lives in terminals and wants a reliable
proxy panel while coding.

Core jobs:

- See at a glance whether the proxy is actually working.
- Know which node/group is active and whether latency is acceptable.
- Switch nodes with low friction.
- Find and close noisy connections.
- Inspect logs and rules when routing is wrong.
- Toggle runtime settings only when necessary.

Non-goals:

- Do not make the TUI the only management surface.
- Do not hide important state behind modal-only interactions.
- Do not duplicate complex business logic that already exists in CLI/client code.
- Do not design a decorative landing screen.

## 3. Design Direction

Use a restrained operations-console aesthetic:

- Dense, scannable, and calm.
- Dark-first, but readable on light terminals.
- Few borders, strong alignment, clear active states.
- Status colors only for meaning, not decoration.
- Tables should look like tools, not forms.

The redesign should keep the five-view mental model but avoid a permanent left
navigation rail. A left rail makes the product feel like a web admin panel and
steals space from the data tables that matter most. Use top tabs plus
view-specific panes instead.

## 4. Information Architecture

Use a top-tab command workspace instead of a permanent left navigation rail.
The previous left-rail layout is easy to understand but wastes horizontal space
and makes the TUI feel like a generic web dashboard squeezed into a terminal.

Preferred global layout:

```text
 LabProxy  ● running  mode:rule  sys:on  tun:on  conns:42  ↓1.2MB/s ↑86KB/s
 ─────────────────────────────────────────────────────────────────────────────
 [1 Proxies] [2 Connections] [3 Logs] [4 Rules] [5 Config]   ctrl:127.0.0.1:9090

 Groups                         Nodes
 ▌ OpenAI       JP 04  42ms      ▌ JP 04              42ms   active
   GitHub       JP 04  43ms        JP 01              78ms
   Proxies      JP 01  78ms        SG 02             121ms
   Telegram     SG 02 121ms        US 03             268ms
   Netflix      US 03 268ms        HK 01           timeout

 Details
 OpenAI → JP 04 · last tested 23:35 · used by chatgpt.com, api.openai.com
 Recent: codex → chatgpt.com matched OpenAI via JP 04
 ─────────────────────────────────────────────────────────────────────────────
 / filter  j/k move  h/l pane  enter switch  t test group  r refresh  ? help
```

Layout rules:

- Header is always one line when possible and summarizes live health.
- Top tabs replace left navigation to preserve horizontal room for tables.
- Main region is a stable two-pane or table workspace depending on the view.
- A compact detail strip sits below the main workspace for selected-row context.
- Header should show runtime health and transport state only. Do not put a
  single active node in the global header because different proxy groups can
  resolve to different nodes.
- Detail strip is contextual only: max two lines, read-only, no independent
  scrolling, and no primary actions.
- Footer is contextual and shows only useful keys for the active view.
- No nested cards. Use section labels, spacing, and subtle separators instead.
- Avoid all-modal workflows for primary tasks.

Responsive rules:

- Width `< 90`: hide tabs' descriptive labels, keep `[1] [2] [3] [4] [5]`.
- Width `< 100`: Proxies becomes a single list with group filter above it.
- Height `< 22`: hide the detail strip first, then secondary columns.
- Width `>= 144`: detail strip may become a right inspector capped at 28
  columns. Between `132-143`, stay in Standard mode to protect the middle pane.
- Do not use a permanent side rail at any width.

## 5. Layout Specification

### Global Chrome

Every view uses the same chrome budget:

```text
row 1   header: health, mode, system proxy, TUN, connection count, traffic
row 2   tabs: active view plus controller endpoint when room allows
row 3+  main workspace
last-2  optional detail strip or confirmation banner
last-1  footer: current-view keys only
```

Rules:

- Header and tabs are fixed height. They never wrap.
- Footer is exactly one line. It is hard-truncated, never wrapped.
- If a confirmation is active, it replaces the detail strip first, not the main
  table.
- Detail strip is optional. It appears only when height is sufficient and must
  never reduce the main list below eight visible rows.
- Detail strip content should be stable. Avoid rapidly changing counters or
  recent-activity text that causes visible jitter.
- Filter/search state is rendered inline in the active view's first content row,
  not as a global third chrome row.

### Width Modes

```text
Compact   < 90 columns     single primary list, numeric tabs, no inspector
Standard  90-143 columns   primary split panes, bottom detail strip
Wide      >= 144 columns   primary split panes plus optional 28-column inspector
```

### Height Modes

```text
Short     < 22 rows   header + tabs + main + footer, no detail strip
Normal    22-31 rows  bottom detail strip allowed
Tall      >= 32 rows  more table rows, no new permanent panes
```

Tall terminals should show more rows, not more chrome.

### Proxies Layout

Compact:

```text
Group: OpenAI  / filter

▌ JP 04             42ms   active
  JP 01             78ms
  SG 02            121ms
```

Standard:

```text
Groups                         Nodes
▌ OpenAI       JP 04  42ms      ▌ JP 04              42ms   active
  GitHub       JP 04  43ms        JP 01              78ms
  Proxies      JP 01  78ms        SG 02             121ms

Details: OpenAI -> JP 04 · used by chatgpt.com, api.openai.com
```

Wide:

```text
Groups                  Nodes                              Inspector
▌ OpenAI  JP04 42ms     ▌ JP 04  42ms active              Chain: OpenAI -> JP 04
  GitHub  JP04 43ms       JP 01  78ms                     Last test: 23:35
  Final   JP01 78ms       SG 02 121ms                     Used by: 2 groups
```

Pane sizing:

- Compact: one list, full width.
- Standard: groups `30-36%`, nodes fill remaining width.
- Wide: groups `24-28 cols`, optional inspector `28 cols`, nodes fill the
  middle.

In Standard mode, Proxies has only two active focuses: groups and nodes. The
detail strip is not focusable.

### Connections Layout

Compact:

```text
Host                              Rule      Age
▌ chatgpt.com                     OpenAI    02:12
  api.github.com                  GitHub    00:08
  bohrium.tech                    DIRECT    timeout
```

Standard:

```text
Host                              Rule      Chain      ↓       ↑     Age
▌ chatgpt.com                     OpenAI    JP 04    1.2MB    80KB  02:12
  api.github.com                  GitHub    JP 04    320KB    12KB  00:08
```

Wide:

```text
Process        Host                         Rule     Chain    ↓      ↑    Age
▌ codex        chatgpt.com                  OpenAI   JP 04  1.2MB   80KB 02:12
  gh           api.github.com               GitHub   JP 04  320KB   12KB 00:08

Inspector: id, network, source/destination, upload/download totals, rule payload
```

The table owns the main area. The inspector is read-only, opt-in with `i`, and
only appears in Wide mode.

### Logs Layout

Compact and Standard:

```text
level: info   filter: openai   status: streaming

23:35:57 info     codex -> chatgpt.com matched OpenAI via JP 04
23:36:04 warning  DIRECT bohrium.tech:50001 timeout
```

Wide:

```text
Log Stream                                              Inspector
▌ 23:35:57 info    codex -> chatgpt.com matched OpenAI  level: info
  23:36:04 warn    DIRECT bohrium.tech timeout          paused: no
```

Logs should prefer full-width readability over extra panes. The inspector is
off by default, optional with `i`, and must disappear before log lines truncate
below useful length.

### Rules Layout

Compact:

```text
Rule Sources
▌ Local overrides     protected
  GitHub provider     loaded
  OpenAI provider     loaded

Safe Workflow: inspect · fetch · validate · plan · verify
Advanced: apply
```

Standard and Wide:

```text
Rule Sources                         Workflow
▌ Local overrides   12 protected     ▌ inspect    read local state
  GitHub provider   loaded             fetch      download candidates
  OpenAI provider   loaded             validate   check groups/payloads
  Telegram          missing            plan       preview changes
                                      verify     compare runtime

Advanced: apply writes mixin.yaml and requires backup display + confirmation
```

Rules is read-only-first. `apply` never shares the same visual group as safe
workflow actions. Facts and safe workflow actions should be visually separated
from advanced write actions by spacing and section labels, not just column
position.

### Config Layout

Compact:

```text
Runtime
▌ Status           running 2d18h
  Controller       127.0.0.1:9090

Actions
  Restart runtime  requires confirmation
```

Standard and Wide:

```text
Runtime                         System
▌ Status        running 2d18h    System Proxy    on
  Controller    127.0.0.1:9090   TUN             on
  Mixed Port    7893             LAN             on
  Mode          rule             IPv6            off

Actions
  Restart runtime     requires confirmation
  Open Web UI         http://127.0.0.1:9090/ui
```

Config is an inspect-first settings surface. Mutation requires `x` plus shared
confirmation; `enter` never writes. Runtime facts, system settings, and actions
must be visually separated so a write action cannot feel adjacent to read-only
state.

## 6. Views

### Proxies

Purpose: switch nodes and understand group health.

Primary layout:

- Two main columns: groups on the left, nodes on the right.
- Bottom detail strip: current node chain, latency history, and recent matching
  traffic.
- On wide terminals, the detail strip may move to a right-side inspector.
- On narrow terminals, collapse to one list: selected group at top, nodes below.

Rows:

```text
▌ JP 04             42ms   active   used by 2 groups
  JP 01             78ms            used by 1 group
  SG 02            121ms            used by 1 group
  US 03            268ms            used by 1 group
  HK 01          timeout            unused
```

Visual behavior:

- Active node uses a left bar and `active` label, not an oversized highlight.
- Group membership detail belongs in the detail strip, not in the row. Main rows
  must stay width-stable as selection changes.
- Latency color is semantic:
  - `< 80ms`: success
  - `< 180ms`: warning
  - `< 350ms`: orange
  - `>= 350ms`: danger
  - timeout/error: danger muted
- Search filters node names and visible policy groups.
- `t` tests the selected group; `T` tests all visible groups only if implemented
  later.

### Connections

Purpose: explain current traffic and close problematic connections.

Primary layout: full-width table with optional inspector.

Columns:

```text
Process        Host                         Rule        Chain       ↓       ↑    Age
codex          chatgpt.com                  OpenAI      JP 04    1.2MB   80KB  02:12
gh             api.github.com               GitHub      JP 04    320KB   12KB  00:08
Comet Helper   usih1471763.bohrium.tech     DIRECT      -           0B     0B  timeout
```

Responsive columns:

```text
>= 140 cols   Process Host Rule Chain ↓ ↑ Age
110-139 cols  Host Rule Chain ↓ ↑ Age
90-109 cols   Host Rule Chain Age
< 90 cols     Host Rule Age
```

All omitted fields remain available in the detail strip or wide inspector.

Interaction:

- `/` filters by process, host, rule, or chain.
- `d` closes selected connection after inline confirmation.
- `D` closes all visible connections after inline confirmation.
- `enter` opens a right-side detail panel on wide terminals.

Design priority:

- Host and rule must be more legible than raw ID.
- DIRECT traffic should be visually distinct, especially if it timed out.
- Connection rows should not jump when traffic counters update.

### Logs

Purpose: monitor runtime behavior without tailing files separately.

Layout:

```text
Level: info  Filter: openai

23:35:57 info     TCP codex -> chatgpt.com matched OpenAI via JP 04
23:36:04 warning  DIRECT bohrium.tech:50001 timeout
23:36:13 info     TCP gh -> api.github.com matched GitHub via JP 04
```

Interaction:

- `l` cycles level: info, warning, error, debug.
- `/` filters text.
- `c` clears local buffer.
- `p` pauses/resumes streaming.

Visual behavior:

- Timestamp muted.
- Level colored by severity.
- Repeated noisy lines should remain readable, not dominate the screen.

### Rules

Purpose: inspect routing logic and run safe rule workflows.

Recommended default view:

```text
Rule Sources
▌ Local overrides       12 rules    protected
  GitHub provider       loaded      Proxies
  OpenAI provider       loaded      OpenAI
  Telegram provider     missing     -

Safe Workflow
  inspect       read local rule state
  fetch         download candidates
  validate      check groups and payloads
  plan          preview provider changes
  verify        compare runtime state

Advanced
  apply         writes mixin.yaml, requires backup display and confirmation
```

Interaction:

- Default actions are read-only: inspect, validate, plan, verify.
- `apply` is not a primary key. It should require an explicit command path or a
  confirmation screen because it writes user config.
- Local overrides are visually marked as protected and must stay above generated
  provider rule sets.

### Config

Purpose: change runtime settings safely.

Layout:

```text
Runtime
  Status              running for 2d 18h
  Controller          http://127.0.0.1:9090
  Mixed Port          7893
  Mode                rule

System
  System Proxy        on
  TUN                 on
  LAN                 on
  IPv6                off

Actions
  Restart runtime     requires confirmation
  Open Web UI         http://127.0.0.1:9090/ui
```

Interaction:

- `enter` opens details or expands the selected item. It never writes.
- `x` executes the selected action.
- Any action that mutates runtime or user config shows an inline confirmation
  banner: `Apply TUN=off?  y confirm · esc cancel`.
- Restart must use the same confirmation pattern.
- Show the exact endpoint/port being used. This avoids config guessing.

## 7. Visual System

Keep colors tokenized in `internal/tui/theme/theme.go`. Add semantic tokens
rather than raw inline colors.

Recommended token set:

```text
Bg              base terminal background
Chrome          header / tabs / footer background
Pane            workspace background
SurfaceRaised   selected row / elevated region
SelectionBg     selected row background
SelectionFg     selected row text
Current         active value / current route
BorderSubtle    separators and low-priority boxes
Divider         subtle separators
TextPrimary     important labels and active values
TextSecondary   table secondary columns
TextMuted       timestamps, hints, inactive labels
Accent          active view, selected row bar
Success         running, good latency
Warning         moderate latency, degraded state
Orange          high latency
Danger          failed, timeout, stopped
Info            links, controller endpoint
```

Suggested dark palette using 256-color indexes:

```text
Bg             234
Chrome         235
Pane           234
SurfaceRaised  236
SelectionBg    237
SelectionFg    255
Current         81
BorderSubtle   238
Divider        238
TextPrimary    255
TextSecondary  250
TextMuted      244
Accent          75
Success         42
Warning        220
Orange         214
Danger         203
Info            81
```

Typography rules:

- No viewport-scaled type; terminal text is fixed.
- Use uppercase section labels sparingly.
- Avoid decorative symbols except meaningful status markers:
  - `●` running/active
  - `○` inactive
  - `▌` selected/active row
  - `!` warnings when color may be unavailable

Borders:

- Prefer no border for simple sections.
- Use a single subtle border around the main work area only on wide screens.
- Avoid heavy nested boxes.

## 8. Interaction Model

Global keys:

```text
1-5     switch views
tab     next view
r       refresh
/       filter in current view
?       help
q       quit
```

View keys:

```text
Proxies       j/k move · h/l group/node · enter switch · t test
Connections   j/k move · enter details · d close · D close visible
Logs          j/k scroll · l level · p pause · c clear
Rules         j/k move · enter inspect · v validate · p plan
Config        j/k move · enter details · x execute · R restart confirm
```

Confirmation pattern:

```text
Close 18 visible connections?  y confirm · esc cancel
Apply TUN=off?  y confirm · esc cancel
```

Do not use pop-up modals for routine confirmation. Inline confirmation keeps
context visible.

## 9. Layout Implementation Plan

Implement layout before visual polish. The goal is to change spatial structure
without changing proxy behavior.

Treat this as a small layout-engine rewrite, not a skin pass. The current app
composes `header + body + footer`, and the body subtracts a fixed nav width
before rendering a full-height view. The target design needs one shared layout
calculator driven by `WindowSizeMsg`; views should render into exact rectangles
instead of discovering available space ad hoc.

Step 1: introduce layout modes

- Add a small `layoutMode` helper derived from terminal width/height:
  `Compact`, `Standard`, `Wide` plus `Short`, `Normal`, `Tall`.
- Unit-test threshold behavior at `80x20`, `100x24`, `120x28`, `140x32`.
- Include `144x32` to prove the first Wide inspector threshold.
- Keep current data refresh, proxy switching, logs, and rules behavior intact.

Step 2: replace left nav with top tabs

- Replace `renderNav` usage in `renderBody` with a top-tab renderer.
- Header stays one row; tabs become the second row.
- Header, tabs, and footer use display-width-aware truncation and must never
  wrap.
- Body receives the remaining height after header, tabs, optional detail, and
  footer are accounted for.

Step 3: add detail strip / inspector plumbing

- Add a per-view `detailText()` or equivalent renderer.
- In `Standard` mode, render it as a max two-line bottom strip.
- In `Wide` mode, allow views to render a 28-column inspector, but keep it
  optional and disabled by default for Logs.
- Never place primary actions in detail/inspector.

Step 4: migrate views one at a time

- Proxies first: it is the default screen and proves the split-pane model.
- Connections second: it proves responsive table columns.
- Config third: it proves inspect-first write confirmation.
- Logs fourth: add viewport-style pause/scroll/filter behavior before adding
  any inspector.
- Rules last: it has the biggest mismatch with the current modal-backed
  implementation.

Step 5: test layout as a contract

- Add tests for each view at Compact, Standard, and Wide widths.
- Assert footer text matches active view.
- Assert CJK node names do not shift columns or overflow visible panes.
- Assert short-height mode hides details before hiding primary rows.
- Assert header, tabs, and footer remain one line with long endpoints, long node
  names, and Chinese labels.

## 10. Implementation Priorities

Phase 1: visual polish without behavior change

- Tokenize missing colors.
- Replace crowded boxes with header, top tabs, main workspace, detail strip, and
  footer.
- Improve row selection, active node, latency colors, and footer hints.
- Verify with existing `go test ./internal/tui/...`.

Phase 2: view-specific UX

- Proxies: stable group/node split, search, latency sort.
- Connections: stable columns, filter, detail panel.
- Logs: pause/clear/filter with level indicator.
- Config: inspect-first rows, explicit `x` execution, shared confirmation for
  writes.

Phase 3: rules workflow UI

- Make safe read-only workflow steps first-class.
- Keep `apply` intentionally guarded.
- Show protected local override ordering.

## 11. Acceptance Criteria

- First screen answers: running?, system proxy on?, current mode?, active node?,
  active connections?, current traffic?
- A user can switch a node without reading documentation.
- A user can identify a misrouted or timed-out connection in under ten seconds.
- Footer always matches the active view.
- No view requires remembering hidden keys for primary actions.
- No primary view is implemented as a modal.
- TUI remains a thin human layer over the shared CLI/proxy client behavior.
- Tests pass: `go test ./internal/tui/...`.
- Build passes when implementation changes: `VERSION=dev bash scripts/build-tui.sh`.

## 12. Open Design Questions

- Should `labproxy tui` default to Proxies or a compact Overview screen?
  Recommendation: keep Proxies as default, but make the header strong enough to
  function as the overview.
- Should `apply` for rules exist inside TUI?
  Recommendation: yes, but behind explicit confirmation and backup display.
- Should node sorting be persistent?
  Recommendation: start with local session-only sorting to avoid surprising
  runtime config changes.
