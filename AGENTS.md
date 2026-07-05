# labproxy Agent 工作契约

本文件是根目录级别的 agent instructions。它优先描述 labproxy 这个项目的真实工作方式：本机代理、Mihomo runtime、规则工作流、安装态二进制、用户本地配置和 Git 边界。

## 核心原则

- 先查真实状态，不要猜配置。涉及代理是否正常、谁接管系统代理、规则是否生效、端口是否占用时，必须用 live state 验证。
- 保护用户本地代理配置。默认不要改 `/Users/azhi/.labproxy/*`、`resources/mixin.yaml` 或安装态 runtime 文件，除非任务明确要求。
- 默认只读验证。规则 workflow 的 `candidates`、`inspect`、`fetch`、`validate`、`plan`、`verify` 是安全默认路径；`apply` 不是。
- 小步实现，小步验证。Go 代码、shell 脚本、规则工作流和安装态 binary 各自验证，不用一个信号替代全部结论。
- Git 边界要干净。只提交当前任务相关文件，不顺手提交用户本地改动。

## Live State 优先

回答这些问题前必须实际检查机器状态：

- 代理是否正常
- LabProxy / Clash Verge / 系统代理谁在接管
- 当前出口 IP / 地区
- Mihomo controller 是否可用
- 规则是否执行
- 端口、进程、TUN、mixed-port 是否符合预期

常用只读检查：

```bash
scutil --proxy
env | grep -i proxy
lsof -nP -iTCP -sTCP:LISTEN | grep -E '7893|9090|mihomo|clash|labproxy'
pgrep -af 'labproxy|mihomo|clash|verge'
curl -sS --max-time 5 http://127.0.0.1:9090/configs
curl -sS --max-time 12 -x http://127.0.0.1:7893 https://www.cloudflare.com/cdn-cgi/trace
```

如果多个信号冲突，报告冲突并继续查，不要用单一配置文件下结论。

## 规则 Workflow 安全边界

默认允许这些只读命令：

```bash
labproxy rules workflow candidates
labproxy rules workflow inspect
labproxy rules workflow fetch --candidates=github,openai
labproxy rules workflow validate --endpoint=http://127.0.0.1:9090 --candidates=github,openai
labproxy rules workflow plan --candidates=github,openai
labproxy rules workflow verify --endpoint=http://127.0.0.1:9090
```

默认禁止这些写入动作，除非用户明确要求或任务已经明确包含它们：

- `labproxy rules workflow apply`
- 直接修改 `/Users/azhi/.labproxy/mixin.yaml`
- 直接修改安装态 runtime 配置
- 替换 `/Users/azhi/.labproxy/bin/labproxy-tui`
- 重启、停止或切换 live 代理服务

如果必须执行写入动作：

1. 先备份原文件或原 binary。
2. 记录 backup path。
3. 执行后做 live read-only verification。
4. 在最终报告中说明回滚路径。

安装态 `labproxy` wrapper 会自动注入 active mixin path。只有直接运行 `labproxy-tui` raw binary 时，才需要在 `rules` 后显式传：

```bash
labproxy-tui rules --mixin-config /Users/azhi/.labproxy/mixin.yaml ...
```

## 本地规则和优先级

保护用户手写的本地 override 规则。新增 provider / `RULE-SET` 时，不要把本地优先规则挤到后面。

当前需要特别保护的语义：

- Hugging Face 相关规则仍使用 inline local rules。
- 本地 override 应保持在生成的 provider `RULE-SET` 前面。
- 规则工作流默认第一批候选是 GitHub / OpenAI / Anthropic / YouTube / Netflix / Disney / Telegram。
- AI/media provider 的目标组要映射到现有策略组，例如 `github -> Proxies`、`openai -> OpenAI`。

如果引入新的外部规则源，先验证 URL、payload 格式、rule type、目标策略组是否存在，再考虑 apply。

## 测试和验证标准

按改动范围选择最小但足够的验证：

- Go 规则/工作流改动：

```bash
go test ./internal/rules ./internal/ruleworkflow ./cmd/labproxy-tui
```

- CLI/workflow 改动：

```bash
bash tests/rules_workflow_cli_test.sh
```

- 共享行为、发布前或多模块影响：

```bash
go test ./...
```

- Live 代理状态问题：跑 Mihomo controller、proxy curl 和必要的 `labproxy rules workflow verify`。
- 文档-only 小改动可以不用全量测试，但要至少检查相关命令/路径是否和真实 wrapper 行为一致。
- 不要用 `workflow apply` 作为默认验证手段。

如果测试或 live 验证失败，继续定位并修复。不要在失败状态下声称完成。

## Git / Branch / Review / Merge 流程

开始任何会改文件的任务前必须检查：

```bash
git status --short --branch
git branch -vv
```

### Skills 配合

本仓库允许并鼓励使用用户本地 skills，但 skill 必须服从本文件的 labproxy 安全边界。尤其是 live proxy 写入、安装态 binary 替换、`workflow apply`、重启代理服务，不能因为某个通用 ship/review workflow 要求自动化就绕过本文件的备份和验证要求。

常用路由：

- 代码 diff 自审 / pre-landing review：优先用 `review` skill 或等价只读 reviewer 子代理。
- 准备 push、PR、并入 main、发布交付：优先用 `ship` skill；不要手写一套绕过 review/test 的 push 流程。
- 手动/浏览器/运行态验证：用 `qa` 或 `qa-only`；涉及代理 live state 时仍必须跑本文件列出的 Mihomo controller、proxy curl、端口和进程检查。
- 风险较高的清理、删除、批量修改：先用 `freeze` / `guard` / `careful` 这类保护性 skill 约束范围，再执行。
- 长任务或上下文易丢任务：用 `context-save` / `context-restore` 保存和恢复进度。
- 复杂、多阶段或可并行任务：可以用 native subagents；主 agent 必须复核子代理结论，不能直接把子代理输出当最终事实。

如果 skill 的默认行为和本仓库约束冲突，以本文件为准。例如：`ship` 可以自动 review/test/push/PR，但如果当前在 `main`、工作树含无关本地改动、测试失败、CI 未查看、或需要 live proxy 写入未备份，则必须停止对应危险步骤并报告阻塞。

如果当前在 `main` 上，默认先创建任务分支再改：

```bash
git switch -c codex/<short-task-name>
```

如果当前分支已有未提交改动，先判断是否属于当前任务：

- 属于当前任务：继续在当前分支小步提交。
- 不属于当前任务：保留原样，不要修改、格式化、暂存或提交。
- 当前分支是 `main` 且已有本地领先提交：不要继续把新任务堆到 `main`；创建新分支承接后续工作，最终再走审核合并。

修改过程中保持小步提交。每次提交前必须检查：

```bash
git status --short --branch
git diff --stat
git diff --check
```

默认视为用户本地改动、不要顺手提交：

- `.gitignore`
- `resources/mixin.yaml`
- `/Users/azhi/.labproxy/*`
- 安装态 runtime 产物
- 与当前任务无关的 shell/service/test 改动

只 `git add` 当前任务明确相关文件。提交后再次检查 `git status --short`，确认剩余未提交内容没有被误纳入。

Commit message 遵守本仓库 hook 要求，包括 Lore-style trailers 和需要的 co-author trailer。提交信息必须说明为什么改，不只描述改了什么：

```text
Co-authored-by: OmX <omx@oh-my-codex.dev>
```

### Agent 自审要求

提交前 agent 必须自己完成一次 code review，不要把明显问题留给用户：

- 检查 diff 是否只包含当前任务相关文件。
- 检查是否误改了用户本地配置、安装态 binary、生成产物或无关格式化。
- 检查测试覆盖是否与风险匹配；行为改动优先补 regression test。
- 检查错误处理、回滚路径、live proxy 安全边界是否仍成立。
- 对复杂改动，优先使用独立 verifier / reviewer 子代理做只读复核；主 agent 必须复核子代理结论。

自审发现问题时，继续修复并重新验证。不要在已知失败、已知未复核、或测试未读完的状态下提交。

### 默认自动合并要求

当用户要求更新、提交、push、交付、并入 `main`，或上下文已经明显是在完成当前任务时，agent 必须默认自己完成以下闭环，不要停下来等用户提醒：

1. 自己做 diff 自审，确认只包含当前任务文件。
2. 跑与改动范围匹配的本地验证；文档-only 至少跑 `git diff --check`，会影响 CI 的改动还要跑本仓库 CI 等价命令。
3. 提交到任务分支并 push 到 GitHub。
4. 创建或更新 PR，写清测试结果、风险和未测项。
5. 等待 GitHub PR checks；失败就读失败日志、修复、重新验证、重新 push。
6. PR checks 通过且无未解决 blocker 后，自动合并到 `main`。
7. 合并后验证本地 `main`、`origin/main`、目标提交和 main CI 状态，再报告完成。

只有遇到 destructive/irreversible 操作、外部权限缺失、CI 平台故障、复杂历史分叉、未解决 review blocker，或 live 代理高影响写入缺少备份/验证路径时，才停止并报告阻塞。不要因为“是否要合并”这种已由任务目标隐含授权的普通下一步而询问用户。

### Push / PR / Merge

Push 时优先推当前任务分支，不要默认推 `main`：

```bash
git push -u origin HEAD
```

推送前必须确认：

```bash
git status --short --branch
git branch -vv
```

默认交付路径：

1. 在 `codex/<short-task-name>` 分支完成实现、测试、自审和提交。
2. 运行 `review` 或等价 diff 自审；复杂改动用独立 reviewer/verifier 复核。
3. 运行和改动范围匹配的测试/验证；文档-only 小改动至少跑 `git diff --check`。
4. 运行 `ship` 或按 `ship` 的顺序完成 push/PR 交付。
5. Push 当前分支。
6. 如仓库支持 GitHub PR，创建 PR，并在 PR 描述中写清测试结果、风险和未测项。
7. 验证 PR 分支和 PR checks。PR 分支验证通过只代表“可以合并”，不代表任务完成。
8. 只有当任务明确要求 agent 自动合并，且 CI/测试通过、无未解决 review/blocker、工作树干净时，才合并到 `main`。
9. 如果用户要求交付、合并、push 到 GitHub、并入 main、自动完成，除非用户明确说“停在 PR”，agent 必须继续完成 main 合并、push、main CI 和 main 状态验证。

PR 分支验证标准流程：

```bash
git switch <task-branch>
git fetch origin
git status --short --branch
git merge origin/main --no-edit
# 如果团队/任务明确要求 rebase，才用：git rebase origin/main

# 按改动范围运行本地验证，例如：
go test ./...
bash tests/clashctl_runtime_apply_test.sh
bash tests/rules_workflow_cli_test.sh
git diff --check

git push
gh pr checks --watch
gh pr view --json mergeable,statusCheckRollup,reviewDecision,headRefOid,baseRefName
```

PR 分支验证的通过条件：

- 本地工作树干净。
- 分支已经用最新 `origin/main` 验证过，没有 unresolved merge conflict。
- 本地相关测试/脚本通过。
- GitHub PR checks 全绿。
- Review 已通过或无需外部 review，且没有 unresolved comments/blocker。
- PR 描述包含测试结果、风险和未测项。

PR 验证完成后的合并闭环：

- 如果用户只是要求“开 PR”或“先验证 PR”，可以停在 PR，并明确报告 PR URL、checks 状态和未合并状态。
- 如果用户要求“合并到 main”、“push 到 main”、“交付完成”、“agent 自己审核提交合并”或等价目标，不要停在 PR；继续执行 main 合并闭环。
- 合并后必须验证 `HEAD == main == origin/main`，并确认目标提交同时被本地 `main` 和远端 `origin/main` 包含。
- main push 触发新的 GitHub Actions 时，必须等待该 main CI 完成。main CI 失败时按 CI 失败流程读取日志、定位、修复、重新 push。

### CI/CD 结果检查和失败处理

本仓库使用 GitHub Actions CI。CI 是合并门禁，不是装饰性信号。push 或创建 PR 后，agent 必须主动检查远端 CI 结果：

```bash
gh run list --branch "$(git branch --show-current)" --limit 5
gh run watch <run-id> --exit-status
gh run view <run-id> --log-failed
```

如果当前分支有关联 PR，也要检查 PR checks：

```bash
gh pr checks --watch
```

CI 失败时：

1. 读取失败 job 和失败 step 的日志，定位到具体命令、包、测试名或脚本行。
2. 判断失败是否由当前分支引入；不确定时默认当作当前分支问题处理。
3. 复现在本地可复现的失败，修复后重新运行相关最小测试，再运行完整 CI 等价命令。
4. 提交修复并重新 push 当前分支。
5. 重新等待 CI，直到通过或遇到无法自行解决的外部阻塞。

禁止在 CI 失败、CI 未完成、或未读取失败日志的状态下合并。只有明确是 GitHub Actions 平台故障、依赖源临时不可用、或远端权限问题时，才把它报告为外部阻塞；报告必须包含 run URL、失败 job、失败 step 和已尝试的本地复现命令。

当前 CI 覆盖：

- `go test ./...`
- `bash tests/rules_workflow_cli_test.sh`

当前没有自动发布/安装态替换的 CD。涉及发布 binary、替换 `/Users/azhi/.labproxy/bin/labproxy-tui`、重启服务或 live proxy reload 时，仍按本文件的备份、只读验证和人工高影响边界执行，不能让 CI/CD 自动改用户本机运行态。

本地自动并入 `main` 的标准流程：

```bash
git status --short --branch
git switch main
git pull --ff-only origin main
git merge --ff-only <task-branch>
git push origin main
gh run list --branch main --limit 5
gh run watch <main-run-id> --exit-status

git status --short --branch
git rev-parse HEAD
git rev-parse main
git rev-parse origin/main
git merge-base --is-ancestor <target-commit> main
git merge-base --is-ancestor <target-commit> origin/main
```

如果 `--ff-only` 失败，说明存在分叉或需要人工级别的历史决策。不要强推、不要 rebase 用户分支、不要用 merge commit 绕过；报告阻塞和分叉点。

也可以使用 GitHub 的 `gh pr merge --rebase --delete-branch` / `--squash` / `--merge`，但仍必须在合并后执行本地和远端 main 验证。不能只因为 GitHub 显示 PR merged 就声称完成。

禁止默认执行：

- `git push --force` / `git push --force-with-lease`
- `git reset --hard`
- `git checkout -- <path>` 覆盖用户改动
- 把无关本地改动塞进同一个提交或 PR
- CI 未过或未查看结果就合并

## Subagent / OMX 使用边界

默认 solo execute。只有复杂、多阶段或可并行的任务才使用 subagent / OMX。

适合委派：

- 规则 workflow 多阶段实现
- runtime/debug 与代码修复可并行
- 独立 code review / verification
- 外部规则源调研
- 复杂测试失败定位

不适合委派：

- 查一个文件
- 跑一个命令
- 小文档修正
- 单点 typo 或简单测试更新

Subagent 输出必须由主 agent 复核。不能把子代理结论直接当最终事实。

如果 `multi_agent` MCP 或 OMX transport 挂掉，不要卡住；改用本地命令、已有报告或 CLI fallback 继续推进。

## 用户偏好

- 用户经常要求“先看真实状态”，不要抽象回答。
- 用户说 `grill me`、`和我确认`、`不要假设` 时，先逐题确认关键分支，再写计划或文件。
- 用户希望自动推进明确、低风险、可逆的任务；不要为普通下一步反复问“是否继续”。
- 但对 live 代理写入、安装态替换、重启服务、提交用户配置这类高影响动作，要备份、验证并明确报告。

## 完成报告

最终报告要短，但必须包含真正有用的信息：

- 改了哪些文件。
- 跑了哪些测试/验证。
- live 状态结论，如果任务涉及代理。
- 是否执行了写入动作、备份路径在哪里。
- 还有哪些本地未提交改动不属于本任务。
- 如果 push 了，给出分支和 PR URL。
