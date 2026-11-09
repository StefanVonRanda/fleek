# fleek

A grid-first terminal file manager written in Go. fleek shows real image, video
and PDF thumbnails, Nerd Font icons, a places sidebar and breadcrumb navigation,
with keyboard and mouse file operations and a full-screen terminal image viewer.

Designed for [Ghostty](https://ghostty.org/): thumbnails are rendered through the
Kitty graphics protocol with Unicode placeholders, so they keep working inside
tmux. Terminals without Kitty graphics fall back to colored half blocks.

## Features

- 3:2 thumbnails for images, videos and PDFs
- Nerd Font folder glyph tiles and file-type icons
- Places sidebar with mounted filesystems and free space
- Responsive grid, selection borders, breadcrumbs and history
- Recursive search across the current tree (`/`)
- Copy, cut, paste, rename, trash and new-folder actions
- Right-click context menu, plus keyboard equivalents
- Full-screen image viewer on double-click or `space`, with arrow-key browsing
- Lazy thumbnail loading and a disk cache
- Kitty graphics for Ghostty/Kitty/WezTerm, with a colored half-block fallback

## Requirements

- Linux
- Go 1.27 or newer
- A terminal with a Nerd Font installed
- Optional: `ffmpeg` for video frames
- Optional: `pdftoppm` from poppler for PDF pages

## Install

From a checkout:

```sh
git clone https://github.com/StefanVonRanda/fleek.git
cd fleek
./install.sh
```

`install.sh` builds `fleek`, installs it to `~/.local/bin` (or `$PREFIX/bin`),
and writes a starter config to `~/.config/fleek/config.toml` if you do not have
one yet.

Alternatively use Go directly:

```sh
go install github.com/StefanVonRanda/fleek@latest
```

Run `fleek` with no arguments to open the current directory:

```sh
cd ~/Pictures
fleek
```

Pass a directory to open it, or a file to open its parent with that file selected.

## Keys

| Key | Action |
| --- | --- |
| `←` `↑` `↓` `→`, `hjkl` | move in the grid |
| `enter` / double-click | open folder or file |
| double-click image | full-screen viewer |
| `space` | preview selected image |
| `c` / `x` / `v` | copy / cut / paste |
| `F2` | rename |
| `n` | new folder |
| `Del` | move to trash |
| `m` / right-click | file actions menu |
| `backspace` | parent folder |
| `alt+←` / `alt+→` | back / forward |
| `~` | home |
| `.` | show or hide dotfiles |
| `/` | recursive search |
| `tab` | jump between the grid and sidebar |
| `r` | refresh |
| `?` | help |
| `q` / `ctrl+c` | quit (`ctrl+c` works from any view) |

Mouse clicks select; double-click opens. The wheel scrolls. Sidebar places,
breadcrumbs and toolbar icons are clickable.

Search results show paths relative to the current directory. `esc` closes search
and restores the folder; `enter` opens the selected result.

## Images and tmux

fleek chooses the image renderer at startup:

- Ghostty, Kitty, WezTerm: Kitty graphics with Unicode placeholders
- everything else: colored half blocks

Inside tmux, Kitty graphics require passthrough:

```tmux
set -g allow-passthrough on
set -ga terminal-features ',xterm-ghostty:RGB'
```

The second line advertises truecolor support to tmux. Without it, tmux may
quantize the image-ID colors and thumbnails will not render correctly.

Force a renderer when needed:

```sh
FLEEK_PROTOCOL=halfblock ./fleek ~/Pictures
FLEEK_PROTOCOL=tgp ./fleek ~/Pictures
```

## Configuration

fleek reads `~/.config/fleek/config.toml` (`$XDG_CONFIG_HOME/fleek/config.toml`)
by default. Use `--config PATH` or `FLEEK_CONFIG` to load another file. Settings
are applied at startup; `examples/config.toml` is a complete annotated example.

### Theme

The theme block accepts hex colours:

```toml
[theme]
background = "#1d1c27"
sidebar = "#15141e"
topbar = "#181721"
status = "#13141c"
panel = "#1b1a24"
accent = "#7aa2f7"
selected = "#232a3f"
text = "#c0caf5"
muted = "#9a97aa"
faint = "#565f89"
dimmer = "#3b394d"
hover = "#3a3950"
sidebar_active = "#262536"
sidebar_text = "#a9afd5"
search_background = "#23222f"
```

### Behaviour and layout

```toml
[behavior]
show_hidden = false
sort = "name"              # name, size, modified
directories_first = true

[layout]
sidebar_width = 20         # 14-32
card_width = 25            # 25-40
gap = 2                    # 1-4

[images]
protocol = "auto"          # auto, tgp, halfblock
```

### Extra places

```toml
[[places]]
label = "Code"
path = "~/code"
icon = "code"
color = "#7aa2f7"
```

Available icon names are `home`, `download`, `document`, `music`, `picture`,
`video`, `code`, `trash`, `disk` and `folder`.

### Environment overrides

| Variable | Purpose |
| --- | --- |
| `FLEEK_PROTOCOL` | `tgp` or `halfblock` |
| `FLEEK_CACHE_DIR` | thumbnail cache root (default: `~/.cache/fleek`) |
| `FLEEK_FONT` | path to a Nerd Font used to render large folder glyphs |
| `FLEEK_CONFIG` | alternative config file path |

Thumbnails and preview images are cached under `~/.cache/fleek/thumbs`. The cache
is safe to delete.

## Current limitations

- File operations act on a single selection; multi-select is not implemented yet.
- Linux is the supported platform for mount and font discovery.
- Sixel is not supported. Ghostty uses Kitty graphics; other terminals get half
  blocks.
- Video and PDF thumbnails require the optional external tools above.

## License

MIT
