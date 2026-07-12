# riftctl

Local Riftbound card data tools powered by the Riftcodex API.

## Install

```bash
./install.sh
```

Rebuilds and installs `riftctl` into your Go bin (`GOBIN`, or `GOPATH/bin`).

## Commands

```bash
riftctl update-db
riftctl update-db --skip-images
riftctl search accelerate
riftctl search "Shadow Isles" --set UNL
riftctl search --type Unit --rarity Rare --domain Fury
riftctl search draw --domain Fury,Calm --domain-any --energy 2-4
riftctl inspect unl-060a-219
riftctl inspect UNL-181
riftctl inspect teemo
riftctl inspect UNL-181 --no-image
riftctl inspect UNL-181 --json
```

`update-db` is the only command that uses the network. It stores the full
Riftcodex card payload in SQLite, rebuilds a local FTS5 index, and caches card
images by default.

`inspect` prints a pretty card summary and, in a TTY, renders cached art via
Kitty/Sixel/iTerm2/halfblocks. If the id/ref is missing it searches by name
and inspects the only match, or prompts you to pick when there are several.
If the image is missing it asks whether to run `update-db` (interactive only).
Use `--no-image` to skip art, or `--json` for the raw API payload. In tmux,
enable `allow-passthrough on` for graphics.

## Storage

- DB: `~/.rift-ctl/cards.db`
- Images: `~/.rift-ctl/images/{riftbound_id}.png`

Override the DB path with `--db`; the image cache lives beside that DB.

## Search

Search is offline after `update-db`. The FTS index covers card names, clean
names, set info, rarity, type, domains, rules text, rich text, accessibility
text, flavour, tags, TCGPlayer ID, artist, and metadata flags.

Text query is optional when at least one filter is set.

```bash
riftctl search [query] \
  --set UNL,OGN \
  --type Unit,Spell \
  --supertype Champion \
  --rarity Rare,Epic \
  --domain Fury,Calm \
  --domain-any \
  --energy 2-4 \
  --might 3 \
  --power 1-2 \
  --signature \
  --alt-art \
  --new \
  --limit 20
```

Filters AND together. Within a multi-value filter, values are OR’d.
Domains default to ALL (card must include every listed domain); `--domain-any`
switches to OR. Stat flags accept `N`, `N-M`, `-M`, or `N-`.
