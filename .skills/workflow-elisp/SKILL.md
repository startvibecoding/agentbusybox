# Workflow Elisp

Use this skill when authoring workflow_run source for dynamic workflow mode. Before
running a non-trivial workflow, call workflow_lint on the raw source first; only
call workflow_run after lint returns valid=true.

The workflow_run source must be one complete raw Elisp form. Do not wrap it in
Markdown fences. When invoking workflow_run for long workflows, set the tool
parameter timeoutSeconds explicitly: omit it for the default tool timeout, use a
positive number of seconds for bounded long runs, and use 0 only for intentional
continuous workflows with no agent-level deadline.

## Progressive References

Start with the loaded core rules. Load scenario files only when the task matches
that pattern.

### 1. Core Rules and Skeletons (references/00-core-rules.md) [loaded]
### 2. Research and Investigation Workflows (references/01-research.md) [load on demand]
### 3. Serial and Parallel Composition (references/02-serial-parallel.md) [load on demand]
### 4. Decision Routing and Branching (references/03-decision-routing.md) [load on demand]
### 5. Bounded While Loops (references/04-continuous-loops.md) [load on demand]
### 6. Horizontal Multi-Agent Collaboration (references/05-horizontal-collaboration.md) [load on demand]
### 7. Master-Slave Small Team Workflows (references/06-master-slave-team.md) [load on demand]
### 8. Evaluator-Optimizer Review Passes (references/07-evaluator-optimizer.md) [load on demand]
### 9. Governance and Human Checkpoints (references/08-governance-checkpoints.md) [load on demand]

## Pattern Selection

- Simple ordered task: load serial and parallel composition.
- Broad research or audit: load research workflows.
- Distinct input classes or risk levels: load decision routing.
- Repeated execution with a runtime stop condition: load bounded while loops.
- One-pass draft, critique, and revision: load evaluator-optimizer review passes.
- If a task needs both repetition and critique, use bounded while loops for the
  loop control and use evaluator-optimizer only for the worker prompt criteria.
- Several peer experts checking one problem: load horizontal collaboration.
- One coordinator decomposes work for specialists: load master-slave team.
- High-impact or user-sensitive decisions: load governance checkpoints.

## Non-Negotiable Constraints

- Use workflow_lint before workflow_run for non-trivial generated or edited
  workflow source. It validates Elisp syntax, workflow/phase/agent forms,
  keyword arguments, required prompts, and result references without running
  worker agents.
- workflow, phase, and agent names must be string literals.
- Each agent literal name becomes the runtime worker agent ID with an "agent-"
  prefix, for example (agent "handler-audit" ...) runs as
  agent-handler-audit. Keep agent names unique within a workflow, especially
  inside parallel branches.
- In while loops or dynamic repeated execution, keep the literal agent name
  stable and set :key to a unique instance key such as (format "r%s" i). Keyed
  results are stored as phase.agent[key].
- defun and defmacro only support fixed parameter lists. Do not use &optional,
  &rest, &body, or any argument marker beginning with &.
- Tool lists must be quoted string lists: '("read" "grep" "find").
- Every agent needs a bounded prompt, expected output, and stop condition.
- Do not rely on hidden defaults for safety-sensitive workers: set :mode,
  :tools, :max-iterations, and workflow_run timeoutSeconds explicitly when
  permissions, tool scope, or duration matter.
- Every non-trivial worker should set :max-iterations explicitly. The default
  is 50 worker-agent loop iterations. This caps the worker's model/tool/result
  turns; hitting it fails that worker with max_iterations. It is only suitable
  for small bounded tasks. Use 80-120 for broad read-only scans and 150-250 for
  edit/test/verification workers.
- For long workflows, set workflow_run timeoutSeconds explicitly. Omit it for
  the default tool timeout, set a positive seconds value for bounded long runs,
  and set 0 only for intentional continuous workflows with no agent-level
  deadline.
- Status checker agents used for loop control must return exactly one token
  such as DONE or NEEDS_WORK, with no rationale or suffix.
- Do not simulate loops by writing many numbered phases. Use while only when a
  bounded runtime loop is actually required, and use :key for repeated agents.
