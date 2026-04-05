# AI written Go implementation of the Notify API 

This repository contains an AI-generated Go implementation of the Notify API. To generate the API I followed the following steps:

1. Let Claude Sonnet 4.6 develop a plan on how to reverse engineer the Notify API from the existing Python codebase, including a breakdown of components and tasks.

2. Launched a series of parallel agents that reversed engineered the Notify API by looking at five key areas of the codebase: Layer 1 — Data Model, Layer 2 — API Surface, Layer 3 — Async Tasks, Layer 4 — Business Rules, Layer 5 — Behavioral Spec. Results were written into the `spec/` directory as markdown files.

3. Based on information in `spec/`, Claude Sonnet 4.6 created a detailed implementation plan and generated a series of briefs for each component.

4. Claude Sonnet 4.6 then called `/openspec propose` to generate a task list for each component, which were written into `openspec/changes/` as markdown files.

5. Using GPT-5.4 each changes was implemented with `/openspec implement` calls.

6. Manual code edits were avoided; all code changes were made via the AI implementation process.