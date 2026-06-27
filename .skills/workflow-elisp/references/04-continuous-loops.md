# Bounded While Loops

Single responsibility: show how to write bounded while loops in workflow Elisp.
Use this only when the task truly needs repeated execution with a runtime stop
condition, such as test-fix cycles or repeated search until a coverage threshold.

Do not use this file for one-pass quality review. For draft, critique, and
revise without runtime repetition, use the evaluator-optimizer reference.

Every while loop must have:

- A hard iteration limit.
- A state variable updated inside the loop.
- A clear stop condition.
- A unique :key for repeated logical agents, usually (format "r%s" i).
- A final phase that summarizes the last state.

## Bounded Test-Fix Loop

    (workflow "bounded fix loop"
      (concurrency 1)
      (let ((i 0)
            (status "NEEDS_WORK")
            (last-worker ""))
        (while (and (< i 3) (not (string= status "DONE")))
          (phase "iteration"
            (agent "worker"
              :key (format "r%s" i)
              :mode "agent"
              :tools '("read" "grep" "edit")
              :max-iterations 150
              :prompt (concat
                "Iteration " (format "%s" i) ". Fix only the highest-confidence issue. "
                "Return a concise change summary and remaining risk. Do not return the loop status."))
            (setq last-worker (result-latest "iteration.worker"))
            (agent "checker"
              :key (format "r%s" i)
              :mode "plan"
              :tools '("read" "grep")
              :max-iterations 80
              :prompt (concat
                last-worker
                "\nCheck whether the objective is complete. Return exactly one token: DONE or NEEDS_WORK. No other text.")))
          (setq status (result-latest "iteration.checker"))
          (setq i (+ i 1)))
        (phase "final"
          (agent "summary"
            :mode "plan"
            :prompt (concat
              "Loop stopped after bounded iterations. Final checker status: "
              status
              "\nLast worker result:\n"
              last-worker
              "\nSummarize changes, evidence, and residual risk.")))))

Important: agent names must still be literal strings. Do not write
(agent (concat "worker-" i) ...). Use :key for the per-round instance identity.
Use (result-key "iteration.worker" "r0") for a specific round,
(result-latest "iteration.worker") for the newest round, and
(results "iteration.worker") for the full keyed history.
