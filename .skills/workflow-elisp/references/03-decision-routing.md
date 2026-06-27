# Decision Routing and Branching

Use routing when distinct classes of input need different tools, modes, or
prompts. The current workflow DSL supports Elisp if/cond, but workflow, phase,
and agent names inside branches must still be string literals.

## Risk-Based Route

    (workflow "risk routed task"
      (phase "classify"
        (agent "classifier"
          :mode "plan"
          :tools '("read" "grep")
          :prompt "Classify the request as LOW, MEDIUM, or HIGH risk. Return one label and rationale."))
      (phase "route"
        (if (string= (result "classify.classifier") "HIGH")
            (agent "high-risk-review"
              :mode "plan"
              :tools '("read" "grep" "find")
              :prompt "Perform conservative high-risk analysis. Require evidence and list approval checkpoints.")
          (agent "standard-review"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Perform standard bounded review and return direct recommendations."))))

## Multi-Route with cond

    (workflow "request router"
      (phase "classify"
        (agent "classifier"
          :mode "plan"
          :prompt "Return exactly one token: BUG, DOCS, REFACTOR, or UNKNOWN."))
      (phase "handle"
        (cond
          ((string= (result "classify.classifier") "BUG")
            (agent "bug-handler"
              :mode "agent"
              :tools '("read" "grep" "edit")
              :prompt "Investigate and fix the bug with minimal edits."))
          ((string= (result "classify.classifier") "DOCS")
            (agent "docs-handler"
              :mode "agent"
              :tools '("read" "grep" "edit")
              :prompt "Update docs for the requested behavior."))
          (t
            (agent "fallback"
              :mode "plan"
              :tools '("read" "grep")
              :prompt "Clarify unknown route and recommend next steps.")))))

Prefer routing labels that are exact strings. If classifier output may include
rationale, ask it to put the label on the first line and route conservatively.
