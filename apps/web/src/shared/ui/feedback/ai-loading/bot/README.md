# bloub bot engine (vendored)

Framework-free SVG morphing engine vendored from
[jeremy-prt/bloub](https://github.com/jeremy-prt/bloub) (MIT, commit `main` @ 2026-09-02).

- Files are copied verbatim; only tests were dropped. Keep them dependency-free
  (no React/Vue/DOM imports) so they stay testable and portable.
- `engine.ts` exports `BotEngine`, whose `sample(t)` is a pure function of time;
  the React wrapper lives in `../ai-loading.tsx`.
- Shape/eye/color constants in `profiles.ts` / `decor.ts` were measured off the
  imitated x.ai bot video; the MIT license covers the code, not the imitated
  design (upstream is unaffiliated with x.ai).

Upstream updates: re-copy the changed `src/bot/*.ts` files and re-run the
frontend tests plus a production build.
