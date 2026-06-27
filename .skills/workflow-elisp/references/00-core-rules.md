# Core Rules and Skeletons

## Hard Syntax Constraints

- The first argument of workflow must be a string literal.
- The first argument of phase must be a string literal.
- The first argument of agent must be a string literal.
- Do not generate workflow, phase, or agent names with variables, function calls,
  concat, format, let bindings, or other expressions.
- An agent literal name also determines the runtime worker agent ID: the ID is
  "agent-" plus the literal name. For example, (agent "handler-audit" ...)
  starts worker agent agent-handler-audit.
- Keep agent names unique within a workflow. Duplicate names overwrite result
  keys, and duplicate names running concurrently can collide on the worker agent
  ID.
- If a repeated worker is intentional, keep the agent name literal and stable,
  then add :key with a unique runtime string. For example, inside a while loop:
  :key (format "r%s" i). The stored result key becomes phase.agent[r0].
- defun only supports fixed parameter lists. Do not use &optional, &rest, &key,
  &body, &allow-other-keys, or any parameter marker beginning with &.
- defmacro only supports fixed parameter lists. Do not use &optional, &rest,
  &key, &body, &allow-other-keys, or any parameter marker beginning with &.
- Use keyword/value pairs for agent options. Every agent option key must be an
  unquoted keyword symbol such as :prompt or :tools.
- The :tools value must be a quoted list of string literals, for example:
  '("read" "grep" "find")

Invalid examples:

    (let ((name "scan")) (phase name ...))
    (agent (concat "worker-" suffix) :prompt "...")
    (defun join (&rest parts) ...)
    (defmacro with-worker (&body body) ...)

## Supported Workflow Forms

- workflow_lint validates raw workflow source without invoking worker agents.
- workflow_run executes the workflow after lint passes.
- (workflow "name" body...) defines one workflow run.
- (concurrency n) sets the maximum number of concurrent worker agents.
- (phase "name" body...) groups sequential phases and records phase state.
- (parallel expr...) evaluates independent branches concurrently.
- (series expr...) evaluates branches sequentially.
- (agent "name" :prompt "..." [:key "instance-key"]
  [:mode "plan|agent|yolo"] [:work-dir "..."] [:tools '("read" "grep")]
  [:max-iterations n]
  [:system-prompt-extra "..."]) runs one worker agent and returns its final text.
- (result "phase.agent") returns one prior worker result.
- (result "phase.agent" :key "r0") or (result-key "phase.agent" "r0") returns
  one keyed worker result.
- (result-latest "phase.agent") returns the newest result for a logical worker.
- (results "phase") returns all results from a prior phase as text.
- (results "phase.agent") returns all keyed results for a logical worker.
- (log "message" ...) appends a workflow log entry.

## Minimal Valid Skeleton

    (workflow "auth audit"
      (concurrency 2)
      (phase "scan"
        (parallel
          (agent "gateway"
            :mode "plan"
            :tools '("read" "grep" "find")
            :max-iterations 100
            :prompt "Audit internal/gateway authentication risks. Return file:line evidence.")
          (agent "hermes"
            :mode "plan"
            :tools '("read" "grep" "find")
            :max-iterations 100
            :prompt "Audit internal/hermes authentication risks. Return file:line evidence.")))
      (phase "verify"
        (agent "cross-check"
          :mode "plan"
          :tools '("read" "grep")
          :max-iterations 80
          :prompt (concat
            (results "scan")
            "\nVerify the evidence, reject weak claims, and list final risks."))))

## Hidden Defaults and Inherited Settings

- (concurrency n) defaults to 5 when omitted. It limits concurrent worker agents,
  not the total number of workers.
- :mode defaults to the parent agent mode; if the parent mode is unavailable, it
  defaults to "agent". Use :mode "plan" explicitly for read-only workers.
- :work-dir defaults to the current process working directory. Set it explicitly
  for cross-directory workflows.
- :key is optional. Use it for repeated logical workers, especially inside while
  loops. It may be a string expression, but it must evaluate to a string without
  brackets or leading/trailing whitespace.
- :tools omitted means the worker receives the default tool set for its mode, but
  workflow workers cannot spawn subagents, delegate, or start nested workflows.
  Prefer explicit :tools lists for bounded workers.
- :max-iterations omitted, 0, or negative falls back to 50 worker-agent loop
  iterations.
- There are no DSL options for :model, :thinking-level, :max-tokens,
  :tool-execution-mode, or per-worker :timeout. These are inherited from the
  surrounding configuration or fixed defaults.
- Worker tool calls execute in parallel by default when the model emits multiple
  tool calls in the same turn.

## workflow_run Timeout

workflow_run has a tool-level timeout separate from worker :max-iterations. It is
a tool parameter, not an (agent ...) DSL option. Omit timeoutSeconds to use the
default tool timeout, set a positive number of seconds for bounded long
workflows, and set timeoutSeconds to 0 only for intentional continuous workflows
that should not have an agent-level deadline. Gateway mode may still have an
outer request timeout.

## Agent Iteration Budgets

The default per-worker :max-iterations is 50 worker-agent loop iterations. It
caps the worker's repeated model/tool/result turns; if the worker hits this
limit before producing a final answer, that worker fails with max_iterations and
can fail the phase/workflow. It is a safety limit for small workers, not a good
default for broad audits or repair loops. Set it explicitly:

- Small read-only check: 50
- Broad scan, inventory, or audit: 80-120
- Synthesis over several prior results: 80-120
- Edit/test/fix worker: 150-250
- Final validation that runs multiple commands: 100-150

If a worker may need many tools, prefer a larger :max-iterations over making the
prompt vague. Keep the prompt bounded and tell the worker exactly when to stop.

## Loop Status Rules

When a result drives control flow with string=, the producing agent must return
exactly the compared token. Do not ask for "DONE - reason" or "NEEDS_WORK plus
evidence" if the workflow later checks (string= status "DONE"). Put evidence in
a separate worker result or a final summary phase instead.

## Generation Checklist

- Run workflow_lint on non-trivial workflow source before workflow_run.
- Source starts with (workflow "literal-name" ...).
- Every phase and agent has a literal string name.
- Agent names are unique and suitable for the agent-<name> runtime worker ID.
- Repeated logical agents use :key for unique instances instead of dynamic agent
  names.
- Parentheses and strings are balanced.
- Every agent option is a keyword/value pair.
- Every agent has :prompt.
- :mode, :tools, and :max-iterations are explicit whenever permissions, tool
  scope, or duration matter.
- :tools uses quoted list syntax exactly like '("read" "grep").
- workflow_run timeoutSeconds is explicit for long workflows; it is a tool
  parameter, not an (agent ...) option.
- :max-iterations is explicit for broad scans, edit workers, verification
  workers, and loop workers.
- Prior outputs are referenced with (result "phase.agent"), (result-key
  "phase.agent" "r0"), (result-latest "phase.agent"), or (results "phase").
