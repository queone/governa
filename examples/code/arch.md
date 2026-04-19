# example-code-repo Architecture

## Purpose

Example governed code repository rendered from the template.

## System Summary

Document the system's major components, boundaries, runtime flow, storage model, and external integrations here.

## Current Platform

- Go service

## Major Components

- entrypoints and user-facing surfaces
- core domain or business logic
- storage, messaging, or state boundaries
- external integrations and trust boundaries

## Core Files

- `AGENTS.md`: base governance contract
- `plan.md`: prioritized roadmap and approved direction
- `build.sh`: convenience wrapper for Unix, Linux, and Git-Bash environments
- `prep.sh`: release-staging wrapper that invokes `cmd/prep` to bump versions, insert the CHANGELOG row, delete completed AC files, and print the release command
- `cmd/build/main.go`: Go build helper, included only for Go-based repos
- `cmd/prep/main.go`: Go release-prep helper, included only for Go-based repos
- `cmd/rel/main.go`: Go release helper, included only for Go-based repos
- `docs/development-cycle.md`: workflow from roadmap through release
- `docs/ac-template.md`: acceptance-criteria template for new work
- `docs/build-release.md`: build, test, and release rules

## Data And Control Flow

Describe the main request, job, or publish path from entrypoint to output.

## Architecture Notes

- record stable system decisions here
- prefer durable structure and interfaces over transient implementation detail

## Conventions

- update this document when architecture or major workflow changes materially
- keep implementation detail in code and stable architecture here
