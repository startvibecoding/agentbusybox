# Governance and Human Checkpoints

Workflow workers cannot directly ask the user mid-run. For high-impact tasks,
split the workflow so it produces a decision packet, then the parent agent asks
the user before running a second workflow that edits or executes.

## Decision Packet First

    (workflow "migration decision packet"
      (concurrency 3)
      (phase "assess"
        (parallel
          (agent "benefits"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "List concrete benefits with evidence.")
          (agent "risks"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "List operational, compatibility, security, and rollback risks.")
          (agent "costs"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Estimate implementation, testing, migration, and support cost.")))
      (phase "packet"
        (agent "decision-packet"
          :mode "plan"
          :prompt (concat
            (results "assess")
            "\nProduce: recommendation, alternatives, explicit approval question, and rollback plan."))))

After this workflow returns, ask the user for approval in the main conversation.
Only then run an implementation workflow.

## Governance Checklist

- Prefer plan mode before edits.
- Make approval points explicit.
- Separate decision workflows from execution workflows.
- Include rollback and observability requirements.
- Record unresolved assumptions in the final packet.
