# Horizontal Multi-Agent Collaboration

Use this when peer specialists should independently analyze the same problem and
then reconcile. It is useful for architecture decisions, security reviews,
product tradeoffs, and adversarial checks.

## Peer Expert Panel

    (workflow "expert panel"
      (concurrency 4)
      (phase "positions"
        (parallel
          (agent "security"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Analyze the proposal from a security perspective. Return must-fix risks and acceptable tradeoffs.")
          (agent "maintainability"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Analyze maintainability, ownership boundaries, and future migration cost.")
          (agent "performance"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Analyze runtime, memory, and scaling implications.")
          (agent "product"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Analyze user-facing behavior, migration risk, and support burden.")))
      (phase "reconcile"
        (agent "moderator"
          :mode "plan"
          :tools '("read" "grep")
          :prompt (concat
            (results "positions")
            "\nFind agreements, contradictions, and a final recommendation with confidence."))))

## Voting Variant

Run the same review prompt through independent agents when diversity matters:

    (workflow "three reviewer vote"
      (concurrency 3)
      (phase "vote"
        (parallel
          (agent "reviewer-a" :mode "plan" :tools '("read" "grep") :prompt "Review for correctness. Return PASS or FAIL with evidence.")
          (agent "reviewer-b" :mode "plan" :tools '("read" "grep") :prompt "Review for correctness. Return PASS or FAIL with evidence.")
          (agent "reviewer-c" :mode "plan" :tools '("read" "grep") :prompt "Review for correctness. Return PASS or FAIL with evidence.")))
      (phase "decision"
        (agent "judge"
          :mode "plan"
          :prompt (concat (results "vote") "\nDecide PASS only if evidence supports it."))))
