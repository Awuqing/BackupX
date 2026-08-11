import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

// BackupX 官方站点 — 托管在 GitHub Pages
// https://awuqing.github.io/BackupX/
const config: Config = {
  title: 'BackupX',
  tagline: 'Self-hosted backup orchestration for servers, databases, storage targets and remote agents',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  url: 'https://awuqing.github.io',
  baseUrl: '/BackupX/',

  organizationName: 'Awuqing',
  projectName: 'BackupX',
  deploymentBranch: 'gh-pages',
  trailingSlash: false,

  onBrokenLinks: 'throw',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'throw',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'zh-Hans'],
    localeConfigs: {
      en: {label: 'English', direction: 'ltr', htmlLang: 'en-US'},
      // Keep the published /zh-Hans/ URL while loading the existing zh-CN
      // translation tree. Without path, Docusaurus silently falls back to the
      // English documents because i18n/zh-Hans does not exist.
      'zh-Hans': {label: '简体中文', direction: 'ltr', htmlLang: 'zh-CN', path: 'zh-CN'},
    },
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/Awuqing/BackupX/edit/main/docs-site/',
          editLocalizedFiles: true,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/social-card.png',
    colorMode: {
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'BackupX',
      logo: {
        alt: 'BackupX Logo',
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docs',
          position: 'left',
          label: 'Docs',
        },
        {
          to: '/docs/deployment/docker',
          label: 'Deployment',
          position: 'left',
        },
        {
          to: '/docs/operations/monitoring',
          label: 'Operations',
          position: 'left',
        },
        {
          to: '/docs/reference/api',
          label: 'API',
          position: 'left',
        },
        {
          to: '/community',
          label: 'Community',
          position: 'right',
        },
        {
          type: 'localeDropdown',
          position: 'right',
        },
        {
          href: 'https://github.com/Awuqing/BackupX',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {label: 'Introduction', to: '/docs/intro'},
            {label: 'Quick Start', to: '/docs/getting-started/quick-start'},
            {label: 'Configuration', to: '/docs/deployment/configuration'},
          ],
        },
        {
          title: 'Operations',
          items: [
            {label: 'Monitoring', to: '/docs/operations/monitoring'},
            {label: 'Security', to: '/docs/operations/security'},
            {label: 'Troubleshooting', to: '/docs/operations/troubleshooting'},
          ],
        },
        {
          title: 'Project',
          items: [
            {label: 'GitHub', href: 'https://github.com/Awuqing/BackupX'},
            {label: 'Releases', href: 'https://github.com/Awuqing/BackupX/releases'},
            {label: 'Community', to: '/community'},
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} BackupX · Apache License 2.0`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'yaml', 'ini', 'json', 'go', 'sql', 'nginx'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
