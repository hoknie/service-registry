# Sign-in through an identity provider

Besides email and password, people can sign in with your company's identity provider: any OpenID Connect issuer
(Keycloak, Okta, Azure AD, Google, …) or GitLab, including self-managed GitLab. The login page then shows a button
per provider.

## Setting up a provider

1. Pick a short key for the provider, for example `corp`.
2. Register the registry as an application at the provider, with the redirect (callback) address

   ```
   <PUBLIC_URL>/api/v1/auth/oauth/<key>/callback
   ```

3. Set the variables and restart the server:

```sh
PUBLIC_URL=https://registry.example.com
OAUTH_PROVIDERS=corp                        # several keys, comma-separated
OAUTH_CORP_KIND=oidc                        # or gitlab
OAUTH_CORP_ISSUER=https://sso.example.com/realms/main
OAUTH_CORP_CLIENT_ID=svc-registry
OAUTH_CORP_CLIENT_SECRET=env:CORP_SECRET    # a value, env:NAME or file:/path
OAUTH_CORP_DISPLAY_NAME=Corporate SSO
```

| Variable | Default | Meaning |
|---|---|---|
| `OAUTH_<KEY>_KIND` | — | `oidc` or `gitlab` |
| `OAUTH_<KEY>_ISSUER` | `https://gitlab.com` for `gitlab` | the issuer address |
| `OAUTH_<KEY>_SCOPES` | `openid profile email` | requested scopes; GitLab adds `read_user` |
| `OAUTH_<KEY>_AUTO_PROVISION` | `false` | create unknown people at their first sign-in |
| `OAUTH_<KEY>_ALLOWED_DOMAINS` | — (any) | email domains allowed to be created automatically |
| `OAUTH_<KEY>_GROUPS_CLAIM` | `groups` (`groups_direct` for GitLab) | the claim with the person's groups at the provider |
| `OAUTH_<KEY>_GROUP_MAP` | — | provider groups → registry groups (below) |
| `OAUTH_<KEY>_CONTACT_EDITOR_MAP` | — | provider groups → contact editors of registry groups (below) |
| `AUTH_PASSWORD_LOGIN` | `all` | who may still sign in with a password: `all`, `superadmins` or `off` |

All variables are also listed in [`.env.example`](../.env.example).

## Who gets in

- **Existing people.** The first sign-in links the provider account to the user with the same **verified** email.
  Superadmins are never linked this way: they link their account by hand under **Account → Sign-in methods**.
- **New people.** With `AUTO_PROVISION=true` the registry creates them, only for the `ALLOWED_DOMAINS` if set.
  Without it, an unknown person is refused.
- **Several methods.** Under **Account → Sign-in methods** everyone can link more providers or set a password. The
  last remaining method cannot be removed.

## Groups from the provider

`GROUP_MAP` keeps memberships in registry groups in line with the provider at every sign-in. Each rule is
`<provider group>=><registry group>`, separated by `;`; a lone `*` creates a registry group per provider group:

```sh
OAUTH_CORP_GROUP_MAP=platform-team=>Platform;*
```

- Memberships the provider gave follow it: they appear and disappear with the provider's groups.
- Members added by hand stay. Adding by hand a person the provider gave makes that membership manual.
- A membership the provider gave cannot be removed by hand.
- Administration rights and roles of the groups apply to such members as to anyone ([access.md](access.md)).

`CONTACT_EDITOR_MAP` grants the right to edit a group's contacts and on-call link ([owners.md](owners.md)) the same
way, only in groups the person belongs to, and without `*`:

```sh
OAUTH_CORP_CONTACT_EDITOR_MAP=platform-leads=>Platform
```

A right granted by hand stays. A right granted by the provider cannot be removed by hand.
