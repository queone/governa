---
name: governa is one-off template, no sync mechanism
description: governa applies to consumer repos once; consumers own their copies after — fixes don't auto-propagate, advisory log is the cross-repo orchestration mechanism
type: project
---

governa is applied to consumer repos as a one-off operation. There is no sync mechanism back to upstream governa. After application, consumers fully own and manage all formerly-governa files. The Local Rules mechanism that previously tracked consumer-local divergence was retired in AC22.

**Why:** Deliberate trade-off — consumers own their copies in exchange for not paying multi-repo sync-coordination cost. The user oversees governa and multiple consumer repos and is the steward across the family.

**How to apply:**
- When a bug is surfaced in formerly-governa code from a consumer repo, the canonical fix lands in governa, but does NOT auto-propagate. Each consumer needs its own AC to adopt the fix.
- Do NOT reason as if "fix the overlay, consumers inherit." That stopped being true post-sync-retirement.
- An advisory log (location TBD — likely `docs/advisories/` in governa) is being adopted as the orchestration mechanism: discoverer drafts a portable advisory, governa intakes it as canonical record, individual consumer repos consult and adopt on their own ACs.
- First instance of this pattern: `programVersion` regex bug surfaced in utils (their AC26 + IE9), to be advised back into governa.
