# Owners and on-call

Every organization, folder and project can say who is responsible for it: its **owners**. Owners are up to 10 groups
or people; the first one is the primary owner. They are shown on the node's **Overview** with their contacts and
on-call link, sent with notifications, and given to AI agents through MCP.

## Setting owners

1. Open the node, **Settings → Owners** (editors of the node).
2. Choose one of three options:
   - **Inherit** — the node shows the owners of the nearest node above it that has its own.
   - **Set own owners** — replaces the inherited owners for this node and everything below it that has no owners of its
     own.
   - **No owners** — says "nobody owns this" and stops inheritance, even though nodes above have owners.
3. For your own set, start typing in the field or just click it. It suggests groups (with their member count) and
   people (with their email), and narrows the list from the first letter. Reorder with **Move up** and **Move down**; the
   first one is the primary owner.
4. Save.

Owners are inherited as a whole set. The nearest node with its own set wins; sets of different nodes are never
merged. A node without owners shows no owners block at all.

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
