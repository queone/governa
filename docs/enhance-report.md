# Enhance Report

Reference repo: `<reference-root>`

Candidate count: 5

## Summary

- `accept`: 1
- `adapt`: 0
- `defer`: 4
- `reject`: 0

## Candidates

### 1. CODE overlay

- Source: `<reference-root>/README.md`
- Template target: `overlays/code/files/README.md.tmpl`
- Portability: `project-specific`
- Recommendation: `defer`
- Collision impact: `medium`
- Reason: content appears tied to the reference repo and should not be imported directly
- Evidence: headings: skout, Setup, 1. Register a Yahoo Developer App

### 2. CODE overlay

- Source: `<reference-root>/arch.md`
- Template target: `overlays/code/files/arch.md.tmpl`
- Portability: `project-specific`
- Recommendation: `defer`
- Collision impact: `medium`
- Reason: content appears tied to the reference repo and should not be imported directly
- Evidence: headings: Skout Architecture, Overview, System Diagram

### 3. CODE overlay

- Source: `<reference-root>/build.sh`
- Template target: `overlays/code/files/build.sh.tmpl`
- Portability: `portable`
- Recommendation: `accept`
- Collision impact: `medium`
- Reason: workflow helper or release artifact is concrete and portable enough for direct template review
- Evidence: headings: build 2.2.0, Pre-scan for -v / --verbose before positional arg parsing so it never, lands in BUILD_TARGETS and the release syntax ./build.sh TAG MSG is unchanged.

### 4. CODE overlay

- Source: `<reference-root>/plan.md`
- Template target: `overlays/code/files/plan.md.tmpl`
- Portability: `project-specific`
- Recommendation: `defer`
- Collision impact: `medium`
- Reason: content appears tied to the reference repo and should not be imported directly
- Evidence: headings: Skout Roadmap, Product Direction, Interaction Principles

### 5. DOC overlay

- Source: `<reference-root>/README.md`
- Template target: `overlays/doc/files/README.md.tmpl`
- Portability: `project-specific`
- Recommendation: `defer`
- Collision impact: `medium`
- Reason: content appears tied to the reference repo and should not be imported directly
- Evidence: headings: skout, Setup, 1. Register a Yahoo Developer App

