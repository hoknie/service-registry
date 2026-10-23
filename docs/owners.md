# Owners and on-call

Every organization, folder and project can name its **owners**: up to 10 groups or users, the first one being the
primary owner. Owners are inherited down the tree as a whole set — the nearest node with its own record wins, sets of
different nodes are never merged. Set them under the node's **Settings → Owners** (`catalog.write`); the project's
**Overview** shows the owners in force with their contacts, and marks where they come from.

- **Inherit** — no record of its own: the node shows the owners of the nearest ancestor that has one.
- **Own set** — replaces the inherited owners for the node and everything below it that has no record of its own.
- **No owners** — an explicit empty record: it stops inheritance just like a set, so a folder can say "nobody owns
  this" even though its organization has owners. Saving a set replaces it; **Inherit** drops the record.

API: `GET /api/v1/catalog/nodes/{id}/owners` answers the owners in force with `from` (the node of the record), `own`
and `none`; `PUT` takes `{"owners": [{"subject_kind": "group", "subject": "Platform"}], "none": false}` — a group by
name or a user by email, like role bindings. `[]` without `none` returns to inheritance, `[]` with `"none": true`
saves "no owners". Deleting a group or user removes it from every set; a set left empty is deleted, so the node
inherits again. A disabled user stays listed with their status.

**Contacts and on-call.** A group carries up to 10 contacts (email, chat, phone, link) and an on-call link, shown with every
node it owns. A superadmin edits them under **Administration → Groups**, and may grant a member the right to edit
the contacts of that group (**contact editor**); a contact editor edits them under **Account → Groups**. The right
counts only while the person is a member. A login provider can grant it too — `OAUTH_<KEY>_CONTACT_EDITOR_MAP`, see
[sign-in.md](sign-in.md); a right granted by the provider cannot be revoked by hand.

**Finding projects by owner.** The catalog tree view filters by owner (`owner=group:<id>`, `user:<id>` or `me`);
`GET /api/v1/catalog/owners?q=` lists owners of readable projects with counts. Notifications carry the owners of
the project ([notifications.md](notifications.md)), and the MCP tool `get_project` returns them.
