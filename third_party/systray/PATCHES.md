# Local patches to fyne.io/systray v1.12.2

Upstream: https://github.com/fyne-io/systray (Apache-2.0, see LICENSE).

1. `systray_darwin.m` `setMenuItemIcon`: menu item images keep their own size
   (pixels / 2, i.e. @2x) instead of being forced to 16x16 pt. A 32x32 px icon
   still renders at 16x16 pt; wider images (aiquokka's tabular usage rows)
   render at full width.
2. `systray_menu_unix.go` `refresh`: LayoutUpdated signals are coalesced
   (30 ms) instead of emitted once per menu change. Rebuilding the menu used
   to emit one per item, so hosts using libdbusmenu-gtk3 (e.g. Waybar)
   fetched a half-built layout first and showed blank submenus.
3. `systray_menu_unix.go` `GetLayout`: the depth=1 first reply after a menu
   reset now applies to every GetLayout in the first 100 ms, not only the
   first call. Waybar runs one menu per output, all on one connection, so
   with several outputs only one of them got depth=1 and the others showed
   blank submenus.

4. Fork integration: compile this copy as `github.com/iugamlabs/aiquokka/third_party/systray`, rewrite its internal generated-package imports, and move original module manifests to `upstream-go.mod.txt` / `upstream-go.sum.txt`. This avoids a relative `replace` in the root module, which would prevent `go install ...@latest`. All implementation files are gated by `tray` plus supported platforms; D-Bus implementation and dependency tests are Linux-only. The macOS implementation additionally requires CGO. No generator was rerun; generated files only received import paths and build constraints.
