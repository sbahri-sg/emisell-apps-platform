# Invite-only developer onboarding

Emisell App Platform does not expose public developer registration. A trusted Emisell platform operator records a selected candidate, reviews the integration use case and requested scopes, and creates the developer organization only after approval.

## Lifecycle

```text
submitted → under_review → invited → active
     └───────────────→ rejected
                       approved ← revoked invite
                          └────→ invited (new code)
```

- `submitted`: an Emisell operator recorded the candidate.
- `under_review`: identity, business need, extension type, and scopes are being reviewed.
- `invited`: approval created the organization, sandbox entitlement, and a pending owner invitation.
- `approved`: the organization is approved but no invitation is currently pending, usually after revocation.
- `active`: the invited identity accepted the one-time code and received the owner membership.
- `rejected`: the request was declined with review notes retained for the audit trail.

Approval never grants production access. `organization_entitlements.production_access` starts as `false`; a future production launch review must use a separate flow.

## Authorization boundary

Organization roles such as `owner` do not grant access to the review queue. Internal endpoints require a separately signed `platform_operator: true` identity claim. This prevents the owner of one developer organization from approving other developers.

Production JWTs used for invitation acceptance must also carry the verified `email` claim. The invitation email and signed email must match case-insensitively. Request headers cannot override either the operator flag or identity email.

## API workflow

| Method | Path | Who may call it |
| --- | --- | --- |
| `GET` | `/v1/session` | Any authenticated control-plane actor |
| `GET`, `POST` | `/v1/internal/developer-applications` | Emisell platform operator |
| `GET` | `/v1/internal/developer-applications/{applicationId}` | Emisell platform operator |
| `POST` | `/v1/internal/developer-applications/{applicationId}/review` | Emisell platform operator |
| `POST` | `/v1/internal/developer-applications/{applicationId}/approve` | Emisell platform operator |
| `POST` | `/v1/internal/developer-applications/{applicationId}/reject` | Emisell platform operator |
| `POST` | `/v1/internal/developer-applications/{applicationId}/invitations` | Emisell platform operator |
| `POST` | `/v1/internal/developer-invitations/{invitationId}/revoke` | Emisell platform operator |
| `POST` | `/v1/developer-invitations/accept` | Authenticated invited identity |

All state transitions use the last observed `revision`; stale review actions fail with `409`.

## One-time invitation security

- Invitation codes contain 256 bits of cryptographic randomness.
- PostgreSQL stores only the SHA-256 digest; the raw code is returned once after approval or rotation.
- Codes expire after `DEVELOPER_INVITATION_TTL` (`48h` locally, bounded to one hour through seven days).
- Only one pending invitation can exist for an application.
- Rotation revokes the previous pending code before creating another.
- Acceptance verifies pending state, expiration, invited email, authenticated user identity, and application state inside one serializable transaction.
- Successful acceptance creates the organization membership, marks the invitation accepted, activates the application, confirms sandbox access, and appends the audit event atomically.
- API list/get responses never include the raw invitation code or its digest.

Send the one-time code through a previously verified secure channel. Do not place it in query strings, analytics, tickets, chat rooms with broad membership, screenshots, or application logs. If delivery is uncertain, revoke it and issue a new code.

## Operator checklist

Before approval:

1. Verify the legal/business identity and company domain outside the platform.
2. Verify control of the invited business email.
3. Confirm the app use case and extension type.
4. Minimize requested scopes; reject unrelated write access.
5. Record review notes without credentials, tokens, personal documents, or other unnecessary sensitive data.

After acceptance, the developer may work only in the sandbox entitlement. Production credentials, merchant access, and launch approval remain separate controls.
