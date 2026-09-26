# ADR 0005: Simple JWT Login behind `Authenticator` Port

## Status

Accepted

## Context

The system needs to know who is calling (name and role) to enforce the rules. A real identity provider (Keycloak) is planned but would slow down the first round. The code that checks permissions should not change when Keycloak arrives.

## Decision

1. `POST /v1/login` takes `{name, role}` with no password and returns a JWT. This is for development and teaching only.

2. The user ID is `role + ":" + lower-case name` with runs of spaces collapsed, so the same person gets the same cases on every login.

   ✅ "Ann Lee" and " ann  lee" both become `customer:ann lee`

3. Tokens are HS256 JWTs signed with `JWT_SECRET`, valid for 12 hours. `pkg/auth.JWT` implements the driven port `usecases.Authenticator`. Verification accepts only HS256 and requires `exp`.

   ❌ accepting `alg: none` or tokens without `exp`

4. Handlers get the user from `usecases.AuthService.Authenticate`, never by parsing the token themselves.

5. Keycloak will be a new `Authenticator` adapter plus a new ADR that supersedes rule 1.

## Consequences

### Positive

- No identity infrastructure needed to start.
- Every permission check is already written against `models.User`, so Keycloak changes only the adapter and the login screen.

### Negative

- **Anyone can log in as anyone.** This must never be deployed where real customers can reach it.
- There is no logout or token revocation before expiry.
