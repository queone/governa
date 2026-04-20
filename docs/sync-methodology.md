# Sync Methodology

Authoritative adoption methodology referenced by `.governa/sync-review.md` on every sync. The repo agent must follow these steps for every `adopt` item flagged in a sync review.

**Default to adopting template content.** Keep existing content only when it is repo-specific and the reason is documented. Adoptions are non-trivial changes to governance docs — draft an AC before applying them so the work gets scoped and reviewed through the normal development cycle.

The repo agent must follow these steps for every `adopt` item:

1. **Structure pass — match the template shape.**
   - The agent must adopt template section names and ordering unless the repo has a documented reason to diverge.
   - The agent must collapse repo subsections that add formatting but not semantic distinction to match the template's flatter structure.
   - If collapsing would lose genuinely repo-specific detail, the agent must keep it inline under the template's section rather than adding new headings.

2. **Content pass — adopt template wording as the base.**
   - For each section, the agent must start from the template text in `.governa/proposed/<file>`.
   - The agent must layer repo-specific additions (project names, file paths, domain rules) on top.
   - If the template wording covers the same intent with better or more general phrasing, the agent must adopt it and drop the repo's version.
   - The agent must not sacrifice detail that is definitively specific to the repo.

3. **Residual check — minimize future drift.**
   - After edits, each remaining difference from the template must be explainable as repo-specific with a clear reason.
   - If a difference has no repo-specific justification, the agent must adopt the template version.

4. **Role files pass — adopt directory and file renames.**
   - When the template renames or restructures a directory, the agent must migrate rather than maintain a divergent path.

5. **Manifest pass — confirm baseline after adoptions.**
   - Sync has already written the updated manifest and TEMPLATE_VERSION. After applying adoptions, the agent must confirm these baseline artifacts remain correct so the next sync diffs against the right starting point.

6. **Report — explain each decision to the director.**
   - For each `adopt` item, the agent must state one of: **adopted** (with summary of changes), **kept** (with documented repo-specific reason), or **needs director judgment** (with explanation).
   - The agent must not silently skip any `adopt` item. Every item must have a stated disposition.
   - For partial-adopt cases (adopting some template content while preserving some existing content), produce `docs/ac<N>-<slug>-dispositions.md` listing each preserved difference with (1) content kept, (2) template content rejected, (3) repo-specific reason. See `docs/ac-template.md` Companion Artifacts.

7. **Feedback — surface improvements for the governance template.**
   - The agent must note any recommendations that were confusing, lacked sufficient context to evaluate, or didn't account for a common repo pattern.
   - The director routes this feedback to governa DEV and QA to improve future sync output and methodology.
