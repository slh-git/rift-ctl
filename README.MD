# rift-ctl

Local Riftbound card data tools powered by the Riftcodex API.

## Commands

```bash
rift-ctl update-db
rift-ctl update-db --skip-images
rift-ctl search accelerate
rift-ctl search "Shadow Isles" --set UNL
rift-ctl inspect unl-060a-219
rift-ctl inspect UNL-181
```

`update-db` is the only command that uses the network. It stores the full
Riftcodex card payload in SQLite, rebuilds a local FTS5 index, and caches card
images by default.

## Storage

- DB: `~/.rift-ctl/cards.db`
- Images: `~/.rift-ctl/images/{riftbound_id}.png`

Override the DB path with `--db`; the image cache lives beside that DB.

## Search

Search is offline after `update-db`. The FTS index covers card names, clean
names, set info, rarity, type, domains, rules text, rich text, accessibility
text, flavour, tags, TCGPlayer ID, artist, and metadata flags.
