# AC1 Add User Authentication (EXAMPLE)

> This is an example AC document, not an active change request.
> It demonstrates the AC pattern used in this repo. Use `ac-template.md` as the starting point for real ACs.

Add email/password authentication with session-based login so users can access protected resources. Code change — adds endpoints, middleware, and tests.

## Summary

Add email/password authentication with session-based login. Users can register, log in, and log out. Protected routes redirect unauthenticated users to the login page.

## Objective Fit

1. **Which part of the primary objective?** Users need to log in before accessing protected resources.
2. **Why not advance a higher-priority task instead?** This is the highest-priority feature gap blocking the beta launch.
3. **What existing decision does it depend on or risk contradicting?** The app already has a session store; this adds the authentication layer on top.
4. **Intentional pivot?** No — direct roadmap work.

## In Scope

### New files

- `auth/handler.go` — registration, login, and logout endpoints
- `auth/middleware.go` — redirect unauthenticated requests to login
- `auth/handler_test.go` — tests for all endpoints and the middleware

### Modified files

- `router.go` — wire auth routes and middleware
- `docs/api.md` — document new authentication endpoints

## Out Of Scope

- OAuth or social login (deferred to a future AC)
- Password reset flow (separate AC — not needed for beta)
- Role-based access control (tempting but not required for the current milestone)

## Implementation Notes

- Use the existing session store rather than adding a new dependency
- Store hashed passwords only; never log or return plaintext passwords
- Rate-limit login attempts to prevent brute-force attacks

## Acceptance Tests

**AT1** [Automated] — Registration creates a user and returns success.

**AT2** [Automated] — Login with valid credentials creates a session.

**AT3** [Automated] — Login with invalid credentials returns 401.

**AT4** [Automated] — Protected route redirects to login when unauthenticated.

**AT5** [Automated] — Logout destroys the session.

**AT6** [Manual] — Verify login flow works end-to-end in the browser.

## Documentation Updates

- `docs/api.md` — new authentication endpoints section
- `README.md` — add "Getting Started" section covering registration and login
- `CHANGELOG.md` — added at release prep time

## Status

EXAMPLE — not an active AC
