# Research and Investigation Workflows

Use this when the task is discovery-heavy: architecture review, risk audit,
competitive research, incident investigation, or "find all places where..."

The pattern is: split independent research lanes, collect evidence, then run a
verification phase that rejects weak claims.

## Codebase Audit

    (workflow "security research"
      (concurrency 4)
      (phase "scan"
        (parallel
          (agent "entrypoints"
            :mode "plan"
            :tools '("read" "grep" "find")
            :prompt "Find public entrypoints and request validation paths. Return file:line evidence only.")
          (agent "storage"
            :mode "plan"
            :tools '("read" "grep" "find")
            :prompt "Inspect persistence and session storage for trust boundary risks. Return file:line evidence.")
          (agent "tools"
            :mode "plan"
            :tools '("read" "grep" "find")
            :prompt "Inspect tool execution paths for sandbox, approval, and path validation risks.")))
      (phase "verify"
        (agent "cross-check"
          :mode "plan"
          :tools '("read" "grep")
          :prompt (concat
            (results "scan")
            "\nVerify each claim against source. Drop speculative findings. Return prioritized issues."))))

## External Topic Research

For web or document research, split by source class or question, not by arbitrary
page count.

    (workflow "market research"
      (concurrency 3)
      (phase "research"
        (parallel
          (agent "primary-sources"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Review provided primary source files. Extract factual claims and citations.")
          (agent "competitors"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Review competitor notes. Extract positioning, pricing, and gaps.")
          (agent "risks"
            :mode "plan"
            :tools '("read" "grep")
            :prompt "Identify legal, operational, and implementation risks from the provided docs.")))
      (phase "synthesis"
        (agent "brief"
          :mode "plan"
          :tools '("read")
          :prompt (concat
            (results "research")
            "\nWrite a concise brief with source-grounded conclusions and unresolved questions."))))
