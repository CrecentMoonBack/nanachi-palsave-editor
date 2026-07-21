# README

## About

This is the official Wails React-TS template.

You can configure the project by editing `wails.json`. More information about the project settings can be found
here: https://wails.io/docs/reference/project-config

## Live Development

To run in live development mode, run `wails dev` in the project directory. This will run a Vite development
server that will provide very fast hot reload of your frontend changes. If you want to develop in a browser
and have access to your Go methods, there is also a dev server that runs on http://localhost:34115. Connect
to this in your browser, and you can call your Go code from devtools.

## Building

To build a redistributable, production mode package, use `wails build`.

## Artwork (optional)

The editor works fully without any images — pals and items are listed by their
Korean names. Icons are a display nicety on top of that.

No artwork ships with this repository: it belongs to Pocketpair. To put it on
your own machine:

```sh
bash scripts/fetch-icons.sh
```

That fills `assets/icons/` (~2462 `.webp` files, ~34 MB) from a
palworld-save-pal checkout over SSH. The folder is gitignored. Keep it beside
the executable in a built release, or in the repo root for `wails dev`; the app
finds it either way, and falls back to text names when it is absent. See
`docs/THIRD_PARTY.md`.
