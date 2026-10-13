# Phase 6: Core-value cleanup - Discussion Log

> Human reference only (audits, retrospectives). Not consumed by downstream agents; decisions live in 06-CONTEXT.md.

**Date:** 2026-10-12
**Areas discussed:** W4 scope disclosure, W2 help-token parsing, W3 empty projects, W1 in scope

## W4 scope disclosure
| Question | Options | Selected |
|---|---|---|
| How should JSON disclose built-in scope? | `scope` object; free-text `note`; help text only | `scope` object |
| Disclose only, or add widening flags? | Disclose only; add widening flags | Disclose only |
| `--hide-done` "(off by default)" help? | Reword; leave | Reword |
| Which reads carry `scope`? | Every read, null when unscoped; only scoped reads | Every read, null when unscoped |
| Human output? | One line on stderr; JSON and help only | One line on stderr |

## W2 help-token parsing
| Question | Options | Selected |
|---|---|---|
| Where is a help token help? | Only in flag position; also drop bare `help` | Only in flag position |
| Fix write-side WR-05 too? | Yes, same fix; reads only | Yes, same fix |

## W3 empty projects
| Question | Options | Selected |
|---|---|---|
| Where to reject? | config.Validate (CLI + TUI); CLI only | config.Validate |
| Reject blank entries? | Yes; only an empty list | Yes |

## W1 in scope?
| Question | Options | Selected |
|---|---|---|
| Include W1? | Yes, panic mid-write exits 5; keep in backlog | Yes |
| Best-effort `unknown` audit on panic? | Yes, best-effort; exit code only | Yes |

## Claude's Discretion
- `scope` key names and stderr wording; how "request issued" reaches the panic handler; where `scope` is attached (envelope vs emit layer).

## Deferred Ideas
- Widening flags (v2 DATA-01); remaining audit tech debt W5, W6, orphaned exports.
