# AI and Media Rules Workflow Design

## Goal

Create a cautious workflow for testing and merging high-quality external proxy rules into labproxy for AI/developer tools and media services.

The workflow must preserve the current working labproxy setup, keep local override rules at the top, and avoid replacing the existing subscription-generated rule base in one large change.

## Current State

The active labproxy runtime uses mihomo in `rule` mode with TUN enabled. Traffic enters through `mixed-port` 7893 and the TUN interface.

Current rule shape:

- `~/.labproxy/mixin.yaml` contains 5 user-maintained override rules.
- `~/.labproxy/runtime.yaml` currently loads 4237 effective rules after merge.
- No `rule-providers` are currently active.
- The effective final fallback is `MATCH,Final`.

Current high-priority user rules:

```yaml
rules:
  - PROCESS-NAME,Cursor,JP
  - DOMAIN,api64.ipify.org,DIRECT
  - IP-CIDR,172.21.0.0/16,DIRECT,no-resolve
  - DOMAIN-SUFFIX,huggingface.co,US
  - DOMAIN-SUFFIX,hf-mirror.com,DIRECT
```

Current known strategy groups that the workflow may target:

```text
OpenAI
Proxies
HK
JP
US
YouTube
Netflix
Disney
Telegram
Google
Apple
Final
DIRECT
REJECT
```

## Scope

The first rule-import workflow targets two categories.

AI and developer tools:

- OpenAI / ChatGPT
- Anthropic / Claude
- GitHub and GitHub asset hosts
- Hugging Face

Media and common services:

- YouTube
- Netflix
- Disney
- Telegram

Apple rules are intentionally not part of the first import batch. Apple traffic is broad, OS-integrated, and easy to over-route. It can be added later after the workflow proves stable.

## Recommended Approach

Use `rule-providers` plus top-level `RULE-SET` references for external rule sources.

Local hand-written rules stay at the top of `mixin.yaml`. External sources live below those local overrides and before broad fallback rules. This preserves user intent while making imported rules easier to inspect, update, disable, or remove.

Avoid bulk-pasting thousands of external rules into `rules:`. Large inline imports are harder to review and can hide duplicates or strategy-group mismatches.

## Candidate Sources

Prefer these sources for the first iteration:

- MetaCubeX `meta-rules-dat` for mihomo-native MRS and geosite/geoip data.
- SukkaW rulesets for curated Mihomo-compatible rule-provider examples.
- blackmatrix7 `ios_rule_script` for service-specific Clash rule files.

Selection criteria:

- The source has a clear raw URL.
- The source format matches the provider behavior: `domain`, `ipcidr`, or `classical`.
- The source is service-specific, not a full replacement rule stack.
- The rule count is reasonable enough to review in a first pass.
- The target strategy group already exists in labproxy.

## First Batch Mapping

Initial provider names and target groups:

```text
github       -> Proxies
openai       -> OpenAI
anthropic    -> OpenAI
huggingface  -> US
youtube      -> YouTube
netflix      -> Netflix
disney       -> Disney
telegram     -> Telegram
```

`hf-mirror.com` remains a direct local override and must stay above imported Hugging Face rules.

## Workflow

1. Snapshot the current state.

   Capture `~/.labproxy/mixin.yaml`, `~/.labproxy/runtime.yaml`, `/rules`, `/proxies`, and a small `/connections` summary.

2. Fetch candidate rule sources into a temporary review directory.

   Do not write to `mixin.yaml` yet.

3. Validate each candidate.

   Check:

   - HTTP status is successful.
   - Size is below the expected limit.
   - Format parses as the expected behavior.
   - Rule count is reasonable.
   - Provider name is unique.
   - Target strategy group exists.

4. Generate a proposed `rule-providers` block and `RULE-SET` additions.

   Keep these additions separate from the existing local override rules.

5. Apply changes to `mixin.yaml` with a timestamped backup.

   Use the existing rules store and provider APIs where possible so write behavior remains consistent with labproxy.

6. Rebuild and hot reload.

   Use the existing service-mode hot reload path:

   ```text
   PUT /configs?force=true
   ```

   TUN creation/removal still requires service restart, but ordinary rules and provider changes should hot reload.

7. Verify runtime state.

   Check:

   - `mihomo` control API responds.
   - `/rules` includes the expected `RULE-SET` entries.
   - `rule-providers` are present in the runtime config.
   - Each provider refresh succeeds.
   - Existing local override rules remain at the top.

8. Run domain probes.

   Suggested probes:

   ```text
   chatgpt.com        -> OpenAI
   anthropic.com      -> OpenAI
   github.com         -> Proxies
   raw.githubusercontent.com -> Proxies
   huggingface.co     -> US
   hf-mirror.com      -> DIRECT
   youtube.com        -> YouTube
   netflix.com        -> Netflix
   disneyplus.com     -> Disney
   telegram.org       -> Telegram
   ```

   Where the API does not expose a direct rule-match endpoint, generate short controlled connections and inspect `/connections` chains.

9. Keep or roll back.

   Keep the changes only if all selected probes match the intended strategy groups and general egress still works through `127.0.0.1:7893`.

## Rollback

Rollback must restore the previous `mixin.yaml`, rebuild runtime config, and hot reload via the same service path.

The workflow should print the exact backup path it created. If a provider import breaks parsing or routing, restoring that backup is the first recovery path.

## Acceptance Criteria

- Existing local override rules remain enabled and first in rule order.
- At least the AI/developer providers pass validation before any media providers are added.
- No imported rule targets a missing strategy group.
- Runtime hot reload succeeds without restarting the root mihomo process.
- `/rules` and `/connections` prove that representative domains route to the expected groups.
- A rollback path is available and tested against the created backup.

## Non-Goals

- Replace the existing 4237-rule subscription-generated base.
- Import every available rule from a third-party repository.
- Add Apple routing changes in the first batch.
- Change node subscriptions or proxy-group membership.
- Change TUN, DNS, or system proxy behavior.

## Risks

- Large external rulesets may duplicate existing inline rules.
- Some repositories publish multiple formats; choosing the wrong behavior can break provider parsing.
- Service-specific rules can be too broad and unintentionally affect local app behavior.
- Current CLI has limited help output and no explicit dry-run mode for provider imports, so the first implementation may need a wrapper script or a new dry-run command.

## Implementation Notes

Prefer implementing the workflow as a repo script or CLI subcommand before changing live config. The first implementation should support:

- `inspect`: print current local rules, provider count, rule count, and strategy groups.
- `fetch`: download candidate provider files to a temporary directory.
- `validate`: parse and summarize candidate providers.
- `plan`: render the proposed `mixin.yaml` diff without applying it.
- `apply`: create backup, write changes, rebuild runtime, hot reload.
- `verify`: run control API and domain-routing probes.
- `rollback`: restore a named backup and hot reload.

This keeps experimentation repeatable and makes future rule updates safer.
