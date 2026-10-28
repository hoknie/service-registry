# Users, groups and rights

Who may do what in the registry. There are two kinds of rights:
- **catalog roles** on nodes — viewer, editor, admin ([catalog.md](catalog.md#roles));
- **administration rights** for the global sections under **Administration**.

## Superadmins

A superadmin may do everything: the whole catalog, every administration section, every user's tokens. The first
superadmin is created at installation ([install.md](install.md)). More are made on the user's page with the
superadmin switch.

## Users

Superadmins, and members of groups with rights on **Users**, manage people under **Administration → Access → Users**: create a user, change the name and email, reset
the password, disable the account. A disabled user cannot sign in; their sessions, tokens and connected apps stop
working. People can also arrive through a login provider ([sign-in.md](sign-in.md)).

**Service accounts.** For integrations, create a user with **Service account** turned on (a password is optional),
or turn it on for an existing user.
- A service account cannot sign in with a password or through a login provider; turning the switch on closes its
  sessions.
- It works only with tokens. A superadmin issues them on the user's page (**Issue token**); the token is shown once.
- Its rights are its roles and groups, like anyone's.

## Groups

Groups collect people so roles, ownership and rights can be given once. Superadmins, and members of groups with
rights on **Groups**, manage them under **Administration → Access → Groups**.

- **Members.** Add and remove people on the group's page: start typing a name or email, or pick from the list. A login
  provider may keep some memberships in sync ([sign-in.md](sign-in.md)). Such a membership cannot be removed by hand;
  adding a person by hand makes the membership manual.
- **Contacts and on-call.** A group has up to 10 contacts and an on-call link, shown wherever the group owns a node
  ([owners.md](owners.md)).
- **Roles.** Give the group a role on any node under that node's **Settings → Access**.

### Groups with roles in the catalog

Once a group has a role on any node or owns one (owners are editors, [owners.md](owners.md#what-owners-can-do)), its
members decide who gets into those nodes. So the members of such a group are changed only by:
- a superadmin;
- the owners of **every** node where the group has a role or is an owner — owners in person or through an owner group
  they belong to. Owners add up down the tree, so the owner of an organization is an owner of every node in it. The
  members of an owner group are owners of its nodes themselves, so they can bring in colleagues.

The right to update groups is not enough for such a group, and owners need no administration rights at all. Owners
manage these groups under **Account → Groups → Groups I manage**: each group shows the nodes it has roles on or owns, its
members with **Remove**, and a field to add people. On the group's page in the administration a note explains why the
members are locked.

### Groups of an organization

A group can belong to a top-level organization instead of the whole registry. Such a group is managed from the
organization itself, under **Settings → Groups** ([catalog.md](catalog.md#groups-of-an-organization)), and works only
inside it:
- it gets roles and ownership only on nodes of its organization;
- it never holds administration rights;
- login providers never add people to it: provider groups are always registry-wide groups;
- its name is unique within the organization; a registry-wide group or a group of another organization may have the
  same name;
- deleting the organization deletes its groups.

The administration list of groups shows organization groups too, marked with their organization, and so does
**Account → Groups**. An organization group is opened and changed only from its organization's **Settings → Groups**:
in the registry-wide administration a non-superadmin cannot open, rename, delete or change the members of a group that
belongs to an organization. Only a superadmin reaches it from the administration list.

## Members of an organization

Every top-level organization has a list of members on its **Members** tab. Everyone who can read the organization sees
the list; its administrators (and superadmins) change it.

A person becomes a member in one of these ways, shown in the **How** column:
- **Added** — an administrator picked an existing user (search by name or email, at least 3 characters) and a role;
- **Invited** — the person accepted an invitation by email;
- **By a role**, **Owner**, **Through a group** — the person got a role on a node of the organization, became an owner
  of one of its nodes or joined one of its groups; such people are added to the list on their own, and moving a folder
  into another organization brings its people along.

The role in the list (viewer, editor or admin) is the person's role on the organization itself; it can be changed or
removed right in the table. The organization always keeps at least one admin: the last one cannot be demoted or
removed.

Removing a member takes away their roles in the organization, ownership of its nodes and membership in its groups.
People who see the organization through registry-wide groups are not members; they are listed separately under
**See it through global groups**, and removing someone from the list does not touch those groups.

### Invitations

An administrator invites a person by email from **Members → Invite**, choosing the role and the language of the letter.
The registry sends a letter with a link when mail is set up ([install.md](install.md#mail)); without mail, or when
the letter could not be sent, the dialog shows the link to pass on by hand. A link works for `INVITE_TTL_HOURS`
(7 days by default) and only once. **Send again** issues a new link and the old one stops working; **Revoke** cancels
the invitation. An organization has at most 100 waiting invitations, and an address can have only one.

The person opens the link:
- without an account — they create one with a name and password, or sign in through a login provider whose verified
  email matches the invitation; the provider creates the account even when it does not create accounts otherwise;
- with an account for that email — they sign in and accept;
- signed in with another email — the invitation is refused; they sign out and open the link again.

Accepting makes the person a member with the invited role.

## Administration rights for groups

A superadmin can let a group run parts of the administration without making its members superadmins.

1. Open the group under **Administration → Access → Groups**.
2. In **Administration rights**, tick the actions per section and press **Save rights**.

| Section | Actions that exist |
|---|---|
| Users | create, read, update |
| Groups | create, read, update, delete |
| Tokens (all users' tokens) | read, delete |
| Clusters | create, read, update, delete |
| Environments | create, update, delete |
| Link kinds | create, update, delete |
| Global secrets | create, read, update, delete |
| Scans | read |
| Catalog top level | create, read, update, delete |
| Publication | update |
| Global AI skills | create, update, delete |
| Global MCP servers | create, update, delete |

Everyone who is signed in can already read the lists of environments and link kinds.

**Catalog top level** is about organizations at the top of the catalog: **read** shows every one of them in the catalog
by name only — without its content, children or settings, which still need a role; **create** adds them, **update** moves an
organization to or from the top and opens or closes an organization ([catalog.md](catalog.md#open-and-closed-organizations)),
**delete** removes a top-level organization. Whoever creates an organization becomes its
owner ([owners.md](owners.md#what-owners-can-do)); without being a superadmin they also become its admin, so they can
see and set it up. The role can be removed under the organization's
**Settings → Access**.

**Publication** lets members change the publication settings of documentation on any node they can read
([project-documentation.md](project-documentation.md#publication)). Editing a node does not give it. Tokens need the
scopes `read` and `publication:write` for it.

**Global AI skills** lets members create, change and delete skills and folders of the global scope and their git
sources ([skills.md](skills.md)). Skills of an organization need a role on the organization instead. Tokens need the
scopes `write` and `skills:write` for it.

**Global MCP servers** lets members create, change and delete MCP servers and folders of the global scope, set their
proxies and decide who may use the global servers ([mcp-gateway.md](mcp-gateway.md)). Servers of an organization need a
role on the organization instead: editors change them, admins decide who may use them. Tokens need the scopes `write`
and `mcp_servers:write` for it.

How the rights work:
- Every active member gets the union of the rights of all their groups. Groups assigned by a login provider count
  too.
- Changes apply on the next request; nobody has to sign in again.
- A member sees **Administration** in the sidebar with only the sections they have rights in. Buttons for actions
  they lack are hidden.
- Only a superadmin changes a group's rights. Members with rights on **Groups** can read them.

Group rights never raise anyone's rights. Without being a superadmin you cannot:
- grant the superadmin flag;
- change the password of a superadmin, of someone who holds rights, or your own through the administration;
- change a group that holds rights;
- rename or delete a group that has roles in the catalog or owns a node — only a superadmin may, since deleting it would
  drop those roles and ownership;
- reach a group that belongs to an organization through the registry-wide group routes — manage it from the
  organization instead;
- change the members of a group with roles in the catalog unless you own every node it has roles on;
- add yourself to a group;
- revoke the tokens of superadmins or of people who hold rights.

Rights are granted one by one and never come bundled. A page shows only what your rights allow: without **Global
secrets: read** the cluster form shows the name of the cluster's current secret and cannot switch to another one.
Adding members to a group needs no right to read users: the member field only offers people to a person who may change
that group.

Personal tokens may carry these rights too; see [access-tokens.md](access-tokens.md).
