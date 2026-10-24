# The catalog

The catalog is a tree: **organizations** at the top, **folders** inside them, **projects** at the leaves. A project is
usually one repository or service. Everything else in the registry hangs off this tree: rights, owners, links,
documentation settings, notifications.

## Finding your way

- **Catalog** in the sidebar opens the top level. The crumbs in the top bar open lists of siblings and children, so
  you can jump across the tree without going back.
- **Ctrl/⌘ + K** opens quick jump: type part of a name or slug. With an empty field it shows the nodes you opened
  recently.
- The contents of an organization or folder are shown as **Cards**, a **Table** or a **Tree**. The tree view loads
  branches as you expand them and filters the whole subtree by name, kind, label, job state or owner.

A project has the tabs **Overview**, **Documentation**, **Details**, **Releases**, **Dependencies** and **Settings**.
An organization or folder has **Overview**, **Details** and **Settings**.

## Creating, moving, deleting

1. Open the organization or folder and press **Add**, then pick **New folder** or **New project**.
2. Fill in the name and the slug (lowercase letters, digits and hyphens; it becomes part of addresses such as
   `acme/backend/api`). Labels are `key=value` pairs and can be filtered on later.
3. A project may get its repository and documentation source right here; see
   [project-documentation.md](project-documentation.md#sources).

New organizations are created by a superadmin or by people allowed to; whoever creates one becomes its owner, and
owners create folders and projects inside ([owners.md](owners.md#what-owners-can-do)). Editing, moving and deleting a node live in **Settings → General**.
Moving and deleting are in its danger zone. Projects imported from a forge are moved and renamed by the
synchronization only ([forge-sync.md](forge-sync.md)).

## Roles

Rights in the catalog come from **roles** given to a user or a group on a node. A role holds on that node and
everything below it.

| Role | Can |
|---|---|
| viewer | see the node and its contents |
| editor | everything a viewer can, plus change nodes, settings, links and documentation settings, and manage project keys |
| admin | everything an editor can, plus give roles, change node secrets and manage notifications |

- Give roles under **Settings → Access** of a node (admins of the node).
- Owners of a node are its editors, see [owners.md](owners.md#what-owners-can-do). Owners add up down the tree: an
  owner set on an organization stays an owner of everything inside it, and lower nodes can only add more.
- Someone with a role deep in the tree also sees the folders above it, but only as a path to get there.
- A superadmin can do everything everywhere. Who administers the registry itself is described in
  [access.md](access.md).

## Open and closed organizations

A top-level organization is **closed** by default: only people with a role in it see it. An **open** organization is
read by every signed-in user, service accounts included, as if each of them were its viewer:
- its folders, projects, owners, links, releases and documentation, documentation search and the documentation section;
- changes still need a role: an open organization gives nobody the right to edit it.

Tokens follow the same rule. A token without chosen nodes sees open organizations. A token limited to nodes sees them only
inside those nodes.

To open or close an organization, use **Settings → General → Visibility**. The switch is available to:
- superadmins;
- the organization's admins;
- members of groups with the **update** right on **Catalog top level** ([access.md](access.md#administration-rights-for-groups)).

An open organization carries an **Open** badge in its header, in the catalog cards and in the tree. An organization moved
under another node becomes closed.

## Groups of an organization

An organization can keep its own groups of people under **Settings → Groups**, so its admins can hand out roles to whole
teams without asking a superadmin. These groups give roles and ownership only inside the organization; see
[access.md](access.md#groups-of-an-organization) for the rest.

- **Who manages them.** Superadmins, the organization's admins and members of groups with rights on **Groups** who can see the organization create,
  rename and delete them and change their members. An organization admin may add themselves too.
- **Who sees them.** Everyone who reads anything in the organization sees its groups and how many members they have. The
  members themselves are shown only to those who manage the groups.
- **Adding people.** Start typing at least three letters of a name or email; the field offers up to ten people.
- **Using them.** In **Settings → Access** and in the owners of any node in the organization, the organization's groups
  are offered first and marked as organization groups. A group name given there means the organization's group, even if
  a registry-wide group has the same name.
- **Moving.** An organization with groups cannot be moved under another node: delete its groups first. A folder or
  project where the organization's groups have roles or ownership cannot be moved to another organization until those
  roles and owners are removed.

## Branches

Every project keeps a list of branches. They come from three places:
- the forge synchronization, for repositories that changed;
- deployments reported by CI (the branch and commit of the event);
- the project's default branch.

Pick a branch with the branch picker on the project page; the **Releases** tab follows it. Under
**Settings → Branches** you can pin a branch to keep it forever or delete a stale record. Deleting a branch never
deletes deployment history.

| Variable | Default | Meaning |
|---|---|---|
| `BRANCH_STALE_DAYS` | 90 | a branch without activity this long is shown as stale |
| `BRANCH_RETENTION_DAYS` | 30 | a branch gone from the forge is deleted after this; a branch seen only in CI after this plus `BRANCH_STALE_DAYS` |
| `BRANCH_SYNC_MAX_PER_REPO` | 500 | at most this many branches are read per repository |

## What is running right now

Projects show what their background jobs are doing:
- documentation collection;
- indexing for search, with the number of files waiting;
- forge synchronization;
- cluster polling.

| State | Meaning |
|---|---|
| running | a worker is on it now |
| queued | it is due, or **Collect now** was pressed, and waits for a worker |
| failed | the last run ended with an error; the hint explains it |
| — | idle, nothing is shown |

Organizations and folders count how many readable projects below them are running, queued or failed. The page
refreshes this every 5 seconds while something is busy, otherwise every minute, and not while the browser tab is
hidden.

If nothing ever leaves **queued**, check `BACKGROUND_JOBS_ENABLED`: a server started with `false` runs no jobs.
