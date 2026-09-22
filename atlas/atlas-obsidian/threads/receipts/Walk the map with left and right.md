---
type: receipt
thread: thr-20260922-cba7
title: "Walk the map with left and right"
outcome: completed
created: 2026-09-22
---

> [!receipt] Walk the map with left and right · completed
> [Stub](<../stubs/Walk the map with left and right.md>) → Spec → Plan → **Receipt**
> `thr-20260922-cba7` · [Thread](<../archive/Walk the map with left and right.md>) · filed 2026-09-22

## Delivered

`graph.tour` is one path through the map: it starts at the leftmost project and always goes to the nearest project not visited yet, so every project is reached once, each step is a short hop, and the walk never bounces between two neighbors. `→`, `↓`, and Tab step forward along it; `←`, `↑`, and Shift+Tab step back; both wrap. The selection follows the walk's start while the map settles, until the user moves it. The find prompt stays as it was: `/` jumps to the first name that matches as it is typed. Directional selection and the by-name cycle are gone. `docs/usage.md`, the keys panel, and `README.md` say so.

## Verified

`go test ./...` passes. The tour test checks the order on a hand-laid graph, the wrap at both ends, and that a settled sample map is covered in full; the view test walks the whole sample with `→`, back with `←`, mixes the vertical arrows and Tab in, and drives find.

## Not done

Nothing. Uncommitted.
