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
**Account → Groups**.

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

Everyone who is signed in can already read the lists of environments and link kinds.

**Catalog top level** is about organizations at the top of the catalog: **read** shows every one of them in the catalog
by name only — without its content, children or settings, which still need a role; **create** adds them, **update** moves an
organization to or from the top and opens or closes an organization ([catalog.md](catalog.md#open-and-closed-organizations)),
**delete** removes a top-level organization. Whoever creates an organization becomes its
owner ([owners.md](owners.md#what-owners-can-do)); without being a superadmin they also become its admin, so they can
see and set it up. The role can be removed under the organization's
**Settings → Access**.

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
- change the members of a group with roles in the catalog unless you own every node it has roles on;
- add yourself to a group;
- revoke the tokens of superadmins or of people who hold rights.

Rights are granted one by one and never come bundled. A page shows only what your rights allow: without **Global
secrets: read** the cluster form shows the name of the cluster's current secret and cannot switch to another one.
Adding members to a group needs no right to read users: the member field only offers people to a person who may change
that group.

Personal tokens may carry these rights too; see [access-tokens.md](access-tokens.md).
