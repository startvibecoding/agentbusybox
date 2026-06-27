# Evaluator-Optimizer Review Passes

Single responsibility: show a fixed, one-pass quality pipeline where one worker
generates output, another critiques it, and a final worker revises it.

Use this for writing, migration plans, design docs, policy analysis, or complex
search when quality improves through explicit critique. This reference does not define loop control.
Do not create numbered phase simulations for repeated attempts. If runtime
repetition is required, load the bounded while loop reference and keep this
file's role limited to critique criteria.

## Draft, Critique, Revise

    (workflow "proposal refinement"
      (phase "draft"
        (agent "writer"
          :mode "plan"
          :tools '("read" "grep")
          :prompt "Draft the proposal. Include assumptions, tradeoffs, and open questions."))
      (phase "critique"
        (agent "critic"
          :mode "plan"
          :tools '("read" "grep")
          :prompt (concat
            (result "draft.writer")
            "\nCritique against correctness, completeness, operational risk, and testability.")))
      (phase "revise"
        (agent "reviser"
          :mode "plan"
          :tools '("read" "grep")
          :prompt (concat
            "Revise the draft using this critique. Preserve strong parts; fix weak claims.\n\nDRAFT:\n"
            (result "draft.writer")
            "\n\nCRITIQUE:\n"
            (result "critique.critic")))))

## When to Use

- Use this for one expected revision pass.
- Keep critique criteria objective and explicit.
- Put final acceptance criteria in the reviser prompt.
- For repeated attempts, use a bounded while loop in the loop reference.
