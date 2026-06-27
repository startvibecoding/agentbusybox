# Serial and Parallel Composition

Use serial phases when later work depends on earlier output. Use parallel inside
a phase when branches are independent and can be reconciled later.

## Prompt Chaining / Serial Pipeline

    (workflow "design then implement"
      (phase "design"
        (agent "designer"
          :mode "plan"
          :tools '("read" "grep" "find")
          :prompt "Design the minimal change. Return files, behavior, risks, and tests. Do not edit."))
      (phase "implement"
        (agent "builder"
          :mode "agent"
          :tools '("read" "grep" "edit" "write")
          :prompt (concat
            "Implement this plan exactly. Keep edits scoped.\n\n"
            (result "design.designer"))))
      (phase "verify"
        (agent "verifier"
          :mode "plan"
          :tools '("read" "grep")
          :prompt (concat
            "Review the implementation against the design. Report issues only.\n\n"
            (results "implement")))))

## Parallel Sectioning

    (workflow "parallel review"
      (concurrency 3)
      (phase "review"
        (parallel
          (agent "api"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Review API compatibility. Return concrete regressions.")
          (agent "tests"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Review test coverage gaps. Return missing cases.")
          (agent "docs"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Review docs and user-facing behavior mismatch.")))
      (phase "merge"
        (agent "triage"
          :mode "plan"
          :prompt (concat (results "review") "\nDeduplicate and prioritize findings."))))
