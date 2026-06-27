# Master-Slave Small Team Workflows

Use this when a coordinator should decompose work, then specialists execute
bounded assignments. The parent workflow is the real master; worker prompts
should not ask workers to spawn or manage further sub-agents.

## Planner Assigns Specialist Tasks

    (workflow "small team change"
      (concurrency 3)
      (phase "plan"
        (agent "master"
          :mode "plan"
          :tools '("read" "grep" "find")
          :prompt "Decompose the request into API, storage, UI, and test tasks. Return scoped instructions for each role."))
      (phase "execute"
        (parallel
          (agent "api-worker"
            :mode "agent"
            :tools '("read" "grep" "edit")
            :prompt (concat
              "You own API/server code only. Do not edit UI or docs.\n\n"
              (result "plan.master")))
          (agent "storage-worker"
            :mode "agent"
            :tools '("read" "grep" "edit")
            :prompt (concat
              "You own persistence/config/session code only. Do not edit UI.\n\n"
              (result "plan.master")))
          (agent "test-worker"
            :mode "agent"
            :tools '("read" "grep" "edit")
            :prompt (concat
              "You own tests only unless a tiny fixture change is required.\n\n"
              (result "plan.master")))))
      (phase "integrate"
        (agent "master-review"
          :mode "plan"
          :tools '("read" "grep")
          :prompt (concat
            (results "execute")
            "\nReview integration boundaries, conflicts, missing tests, and final risks."))))

Rules:

- Give every worker ownership boundaries.
- Tell workers they are not alone in the codebase.
- Prefer narrow tools for each worker.
- Add a final master-review phase before reporting success.
