# BackupX documentation site

The public documentation is a Docusaurus site with English source documents and a complete Simplified Chinese translation.

## Local development

```bash
npm ci
npm start
```

Use `npm start -- --locale zh-Hans` to preview the Chinese site. The public Chinese URL remains `/zh-Hans/`; its source files live under `i18n/zh-CN/` through the locale `path` mapping in `docusaurus.config.ts`.

## Verification

```bash
npm run typecheck
npm run build
```

The production build renders both locales and fails on broken document links. GitHub Actions publishes `build/` to GitHub Pages after changes reach `main`; do not deploy the site manually from a feature branch.

When adding, renaming, or removing a document:

1. Apply the same change under `docs/` and `i18n/zh-CN/docusaurus-plugin-content-docs/current/`.
2. Update `sidebars.ts` and the translated sidebar labels when a category changes.
3. Use relative links for links between documents so both locale prefixes resolve correctly.
4. Run the full verification commands before opening a pull request.
