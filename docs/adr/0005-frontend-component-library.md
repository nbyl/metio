# ADR-0005: Frontend Component Library

- **Status:** Proposed
- **Date:** 2026-08-14
- **Deciders:** Metio maintainers

## Context

The web frontend (`web/`) is styled with plain TailwindCSS 4. Styling lives in
`web/src/index.css`, which combines a CSS-variable token system with a large custom
`@layer components` library (~40 hand-written classes: `.btn`, `.card`, `.badge`, `.switch`,
`.tooltip`, `.whitelist-*`, `.backup-*`, …). Interactive widgets are hand-rolled with **no
headless UI library**:

- `ui/Tabs.tsx` — 201 LOC of custom roving-tabindex, arrow-key navigation, and ARIA wiring.
- `ui/Tooltip.tsx`, `ui/Switch.tsx`, `ui/Separator.tsx`, `ui/Skeleton.tsx` — bespoke
  implementations.
- `server/UpdateModal.tsx`, `server/DestroyModal.tsx` — modals with custom
  `fixed inset-0` overlays, focus and escape handling.
- Large page components (`ServerDashboard` ~756 LOC, `ServerSetupWizard` ~646 LOC,
  `ProvisioningProgress`, `SetupWizard`) mixing inline utilities with custom classes.

Maintaining custom accessibility behavior and bespoke CSS classes by hand does not scale.
The pipeline is growing more forms, dialogs, and status UI (e.g., the ADR-0004 backup
catalog and restore work, story #423), where off-the-shelf, accessible components pay off
most. The test suite also asserts raw class names and ships 13 snapshot files, making
styling refactors costly and low-signal.

The stack already matches what a Tailwind-based component library expects: React 19, Vite,
Tailwind v4 (CSS-first via `@tailwindcss/vite`, no config file), a `cn()` helper (`clsx` +
`tailwind-merge`), `lucide-react` icons, and `sonner` toasts. `index.css` already uses a
shadcn-convention token palette (`--primary`, `--background`, `--card`, `--chart-*` in
`oklch()` with a `.dark` class variant).

## Decision Drivers

In priority order:

1. **Preserve the current aesthetic** — dark-only, green accent, Geist/Geist Mono fonts.
   No visual redesign is in scope.
2. **Accessibility is first-class** — keyboard navigation, focus management, and ARIA on
   the hand-rolled widgets (Tabs, modals, Switch, Tooltip).
3. **Maintainability** — stop owning ~40 custom CSS classes and bespoke widget behavior;
   rely on a maintained, broadly-adopted component library.
4. **Dependency cost is acceptable** — adding `@radix-ui/*` packages is not a blocker.
5. **Proven Tailwind v4 + React 19 support** — the chosen library must work with the
   current CSS-first setup out of the box.

## Considered Options

### (a) Do nothing — keep plain Tailwind + custom classes

Continue hand-writing the `@layer components` classes and widget behavior.

- Pro: no migration effort.
- Con: **fails driver 2** (every widget re-implements focus/keyboard/ARIA) and
  **driver 3** (the bespoke class library keeps growing); test output stays low-signal.

### (b) daisyUI (Tailwind plugin, semantic CSS classes)

Add daisyUI as a Tailwind v4 plugin (`@plugin "daisyui"`) and swap custom classes for
daisyUI's semantic classes (`btn`, `card`, `badge`, `modal`, `tabs`, …).

- Pro: one-line CSS install; zero new JS dependencies; beats custom classes for static
  styling.
- Con: **still no JS behavior** — Tabs, Tooltip, Switch, and modals keep hand-rolled
  accessibility code (fails driver 2); opinionated `data-theme` preset look fights the
  current green/dark aesthetic (fails driver 1); styling control is bounded by theme
  variables rather than the existing token/`cn()` stack (driver 3 weaker).

### (c) shadcn/ui (copy-paste components on Radix UI) *(chosen)*

Use shadcn/ui: source components are copied into the repo, built on Radix UI primitives,
and styled through the existing Tailwind token system.

- Pro: **Radix-backed** Tabs, Dialog/AlertDialog, Tooltip, Switch, Select replace all
  hand-rolled behavior (driver 2); theming maps 1:1 onto the existing CSS variables and
  `.dark` class (driver 1); `cn()`, `lucide-react`, `sonner` already present; Tailwind v4 +
  React 19 supported by the CLI (driver 5); components are source we own — no runtime
  framework lock-in (driver 3).
- Con: adds several `@radix-ui/*` packages plus `class-variance-authority` (accepted,
  driver 4); copied source is upgraded by re-copying; migration + test rewrite effort is
  real.

## Decision Outcome

**Chosen option: (c) — adopt shadcn/ui as the frontend component library.**

Initialize shadcn/ui for Tailwind v4. Preserve the existing CSS-variable tokens and the
dark theme (already shadcn-conformant). Replace all hand-rolled interactive widgets with
shadcn components and remove the custom `@layer components` classes.

Components to add via the CLI (`npx shadcn@latest add …`): `button`, `card`, `badge`,
`switch`, `skeleton`, `separator`, `tooltip`, `tabs`, `dialog`, `alert-dialog`, `input`,
`select`, `label`, `progress`, `form`.

Forms adopt **react-hook-form + zod** (shadcn's `form` component) to replace the
hand-rolled `useState` form state and validation in `ServerSetupWizard`,
`BackupSettingsPanel`, the whitelist form, and the scheduled-shutdown form.

## Consequences

### Positive

- **Hardened accessibility** — Tabs (roving tabindex), Dialog/AlertDialog (focus trap, ESC,
  overlay), Tooltip, Switch, Select all come from Radix.
- **One styling paradigm** — tokens + component source; the ~40 custom classes are deleted.
- **Real form validation** — react-hook-form + zod instead of hand-rolled `useState`.
- **Higher-signal tests** — behavioral role/name assertions replace raw class-name and
  snapshot assertions (13 snapshots removed).
- **Ready for upcoming UI work** — backup catalog, restore, and clone flows (#423) need
  exactly the dialogs/forms/status components shadcn/ui provides.

### Negative / tradeoffs

- **Dependency footprint grows** — Radix packages, `class-variance-authority`,
  react-hook-form, zod.
- **Upgrade path is re-copy** — shadcn/ui is distributed as source; upgrades mean
  re-copying updated components (documented shadcn model).
- **Migration effort** — systematic rewrite of primitives, pages, and tests; the 80%
  coverage threshold must hold throughout.

## Migration

1. **Initialize** — `npx shadcn@latest init` configured for Tailwind v4 and React; keep the
   existing `index.css` tokens and `.dark` variant.
2. **Primitives** — replace `ui/` components with shadcn `add` output, trimmed to the
   existing variants (Button primary/danger/outline + sizes, Badge online/offline/
   transitioning, Card, Switch, Skeleton, Separator, Tooltip, Tabs). Remove the local
   `Tabs.tsx` implementation.
3. **Widgets and modals** — `UpdateModal` → `Dialog`; `DestroyModal` → `AlertDialog`
   (with dialog title/labelled-by); provisioning progress bars → `Progress`.
4. **Pages** — migrate `Layout`, `Header`, `StatsGrid`, `ServerDashboard`,
   `ServerSetupWizard`, `SetupWizard`, `EmptyState`, `ServerConfigPanel`,
   `BackupSettingsPanel`, `ProtectedRoute`; replace inline utilities with components and
   tokens.
5. **Forms** — migrate `ServerSetupWizard`, `BackupSettingsPanel`, whitelist, and
   scheduled-shutdown forms to react-hook-form + zod.
6. **Cleanup** — delete the `@layer components` block from `index.css`; remove now-unused
   token groups (e.g., the unused `--sidebar-*` tokens).
7. **Tests** — update the eight UI/layout test files; drop the 13 snapshot files; keep the
   80% coverage threshold; preserve the behavior-driven integration tests.
8. **Verify** — `make` build, `cd web && npm run lint && npm run format && npm run test:run`,
   manual dark-theme keyboard/screen-reader pass, `make deploy`, visual check.

## Implementation impact

- `web/package.json` — add `class-variance-authority`, `@radix-ui/*`, react-hook-form,
  zod, `tailwindcss-animate` (as required by shadcn/ui).
- `web/src/index.css` — keep token/theme blocks; remove `@layer components`; add shadcn
  `@theme inline` conventions and base layer as generated by the CLI.
- `web/src/components/ui/` — replace all primitives with shadcn components.
- `web/src/components/server/`, `web/src/components/setup/`, `web/src/components/layout/`,
  `web/src/App.tsx` — migrate pages/widgets to components; delete `Tabs.tsx` custom logic.
- `web/src/lib/` — keep `cn()`; add shadcn-generated `utils.ts` conventions.
- `web/src/components/**/*.test.tsx` + `__snapshots__/` — update/remove tests.
- `docs/adr/README.md` — index entry for ADR-0005.

## Related

- ADR-0004 — backup catalog and restore (#423) builds dialogs/forms/status UI on these
  components.
- `web/src/index.css` — current token system and custom `@layer components` (removed).
- `web/src/components/ui/Tabs.tsx` — hand-rolled tabs (replaced by shadcn `tabs`).
