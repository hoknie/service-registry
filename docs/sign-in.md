# Sign-in through an identity provider

Besides email and password, people can sign in through OpenID Connect providers — any OIDC
issuer (Keycloak, Okta, Azure AD, Google, …) or GitLab, including self-managed. Register the
registry as an application at the provider with the callback
`<PUBLIC_URL>/api/v1/auth/oauth/<key>/callback`, then:

```sh
PUBLIC_URL=https://registry.example.com
OAUTH_PROVIDERS=corp
OAUTH_CORP_KIND=oidc                       # or gitlab (issuer defaults to https://gitlab.com)
OAUTH_CORP_ISSUER=https://sso.example.com/realms/main
OAUTH_CORP_CLIENT_ID=svc-registry
OAUTH_CORP_CLIENT_SECRET=env:CORP_SECRET   # a value, env:NAME or file:/path
OAUTH_CORP_DISPLAY_NAME=Corporate SSO
OAUTH_CORP_AUTO_PROVISION=true             # create unknown users…
OAUTH_CORP_ALLOWED_DOMAINS=example.com     # …only for these email domains
OAUTH_CORP_GROUP_MAP=platform-team=>Platform;*   # IdP groups → registry groups
```

The first sign-in links the provider account to an existing user with the same **verified**
email (a superadmin links theirs by hand under **Account → Sign-in methods**). Groups named by
`GROUP_MAP` follow the provider at every sign-in; members added by hand stay. Who may still use
a password is `AUTH_PASSWORD_LOGIN` — `all` (default), `superadmins` or `off`. All `OAUTH_*`
variables: `.env.example`. Specs: `openspec/specs/access/oauth-login/`,
`openspec/specs/web/oauth-login/`.
