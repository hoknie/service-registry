# Owners and on-call

Every organization, folder and project can say who is responsible for it: its **owners**. Owners are up to 10 groups
or people; the first one is the primary owner. They are shown on the node's **Overview** with their contacts and
on-call link, sent with notifications, and given to AI agents through MCP.

Whoever creates an organization becomes its owner right away. Folders and projects created inside show the
organization's owners as inherited.

Owners add up down the tree. A folder or project has the owners of every node above it plus its own. Owners set above
cannot be removed or replaced lower down: lower nodes can only add more.

## What owners can do

Owners are editors of the node where their set is given and of everything below it. They see these nodes, create
folders and projects, change, move and delete them, and manage project keys. They do not give roles: that still
needs the admin role.

- An owner group gives this to each of its members, for as long as they are members.
- Owners added lower in the tree become editors there too. Owners above keep their rights.
- A token limited to some nodes acts as an owner only inside those nodes.

## Setting owners

1. Open the node, **Settings → Owners** (editors and owners of the node).
2. The inherited owners come first, each marked **Inherited from** the node that set it. They cannot be removed or
   moved here; change them on that node.
3. Add owners of this node: start typing in the field or just click it. It suggests groups (with their member count)
   and people (with their email), and narrows the list from the first letter. Reorder your own owners with **Move up**
   and **Move down**, or remove them.
4. Save.

Up to 10 owners can be set on each node. Someone who is already an owner above is not added again. The first owner in
the list — the first one set on the highest node — is the primary owner. A node without owners shows no owners block
at all.

Earlier versions let a node replace its inherited owners or say "no owners". After the upgrade such nodes show the
owners above them again, next to their own.

## Contacts and on-call

A group carries up to 10 contacts (email, chat, phone, link) and an on-call link. They are shown with every node the
group owns.

- Superadmins and groups with rights on **Groups** edit them under **Administration → Access → Groups**.
- The same people can make a member a **contact editor** of the group. A contact editor edits the contacts under
  **Account → Groups**. The right counts only while the person is a member.
- A login provider can grant the right too ([sign-in.md](sign-in.md#groups-from-the-provider)). Such a right cannot
  be revoked by hand.

## Finding projects by owner

The **Tree** view of the catalog filters by owner: a group, a person, or **me**.
