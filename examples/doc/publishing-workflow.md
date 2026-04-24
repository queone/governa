# Publishing Workflow

## Purpose

This file defines how `example-doc` content moves from draft to published output.

## Platform

- Hugo

## Workflow

1. choose the next item from `content-plan.md`
2. draft or revise content to match `style.md`
3. review for clarity, factual accuracy, and consistency
4. publish through the target platform workflow
5. record follow-up updates or future revisions in `content-plan.md`

## Publishing Rules

- keep content changes and planning updates aligned
- do not publish unfinished structural changes without explicit intent
- update this workflow when the platform or editorial process materially changes

## Editorial Variants

This repo includes two pairs of editorial docs. Keep whichever fits your model and delete the other:

- `style.md` (formatting and standards) or `voice.md` (persona and audience) — pick one
- `content-plan.md` (priority-ordered backlog) or `calendar.md` (date-driven schedule) — pick one

## Platform-Specific Notes

Customize the guidance below based on your publishing platform. These are starter notes — replace or expand them as your workflow matures.

**Hugo / Jekyll / static site generators:**
- Content lives in markdown under a `content/` or `_posts/` directory
- Preview locally before publishing; use the platform's build command to verify rendering

**Substack / Ghost / newsletter platforms:**
- Draft in the platform editor or paste from a local markdown file
- Schedule posts against `calendar.md` cadence if using date-driven planning

**WordPress / CMS platforms:**
- Map content-plan items to draft posts in the CMS
- Use the platform's revision history rather than duplicating version control in the repo

**Notion / collaborative docs:**
- Use the repo as the source of truth for editorial standards and planning
- Sync published content back to the repo only when the canonical version matters for review
