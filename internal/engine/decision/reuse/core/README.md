# Reuse Core Compatibility

`internal/engine/decision/reuse/core` is a compatibility layer only.

Purpose:
- Keep legacy import paths usable.
- Re-export symbols from shared modules.

Do not place new implementation here.

Source of truth:
- `internal/engine/decision/shared/model`
- `internal/engine/decision/shared/tool`
- `internal/engine/decision/shared/types`
- `internal/engine/decision/shared/elements`

Rule:
- New code must import `shared/*`.
- `reuse/core/*` should stay thin (alias/wrapper only).
