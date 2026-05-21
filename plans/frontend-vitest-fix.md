# Frontend Vitest Harness Fix

Date: 2026-05-20.

## Context

Previous validation failed before running frontend tests:

```text
TypeError: Cannot read properties of undefined (reading 'config')
```

The frontend uses Vite 8.0.12, Vitest 4.1.6, and Node 25.3.0. The failure was
in the test harness/config loading path rather than in test assertions.

## Fix

Added an explicit `frontend/vitest.config.ts` that keeps Vitest on a minimal
Node test harness:

- `environment: "node"`
- `include: ["src/**/*.test.ts"]`
- project alias `@ -> ./src`
- `css: false`

This avoids loading the full Vite/Vuetify application plugin stack for unit
tests that do not need browser rendering. No dependency downgrade was required.

## Validation

```text
> s-ui-frontend@1.5.2-beta-hotfix2 test
> vitest run

 RUN  v4.1.6 C:/s-ui-main-deposist/frontend

 Test Files  5 passed (5)
      Tests  19 passed (19)
```

Full frontend validation is recorded in [`p1-validation.txt`](p1-validation.txt):

- `npm run lint`: PASS
- `npm test`: PASS
- `npm run build`: PASS
