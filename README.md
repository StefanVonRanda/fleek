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

```sh
go install github.com/StefanVonRanda/fleek@latest
```

Or build from a checkout:

```sh
git clone https://github.com/StefanVonRanda/fleek.git
cd fleek
go build -o fleek .
./fleek ~/Pictures
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

Mouse clicks select; double-click opens. The wheel scrolls. Sidebar places, breadcrumbs and
toolbar icons are clickable.

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

| Variable | Purpose |
| --- | --- |
| `FLEEK_PROTOCOL` | `tgp` or `halfblock` |
| `FLEEK_CACHE_DIR` | thumbnail cache root (default: `~/.cache/fleek`) |
| `FLEEK_FONT` | path to a Nerd Font used to render large folder glyphs |

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
