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

  onBrokenLinks: 'warn',
  markdown: {
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'zh-Hans'],
    localeConfigs: {
      en: {label: 'English', direction: 'ltr', htmlLang: 'en-US'},
      'zh-Hans': {label: '简体中文', direction: 'ltr', htmlLang: 'zh-CN'},
    },
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/Awuqing/BackupX/edit/main/docs-site/',
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
          href: 'https://github.com/Awuqing/BackupX/releases',
          label: 'Downloads',
          position: 'left',
        },
        {
          to: '/community',
          label: 'Community',
          position: 'left',
        },
        {
          to: '/sponsors',
          label: 'Sponsors',
          position: 'left',
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
          title: 'Docs',
          items: [
            {label: 'Introduction', to: '/docs/intro'},
            {label: 'Quick Start', to: '/docs/getting-started/quick-start'},
            {label: 'Installation', to: '/docs/getting-started/installation'},
          ],
        },
        {
          title: 'Features',
          items: [
            {label: 'SAP HANA', to: '/docs/features/sap-hana'},
            {label: 'Multi-Node Cluster', to: '/docs/features/multi-node'},
            {label: 'API Reference', to: '/docs/reference/api'},
          ],
        },
        {
          title: 'More',
          items: [
            {label: 'GitHub', href: 'https://github.com/Awuqing/BackupX'},
            {label: 'Releases', href: 'https://github.com/Awuqing/BackupX/releases'},
            {label: 'Docker Hub', href: 'https://hub.docker.com/r/awuqing/backupx'},
            {label: 'Issues', href: 'https://github.com/Awuqing/BackupX/issues'},
          ],
        },
        {
          title: 'Community',
          items: [
            {label: 'Contributors', href: 'https://github.com/Awuqing/BackupX/graphs/contributors'},
            {label: 'Pull Requests', href: 'https://github.com/Awuqing/BackupX/pulls'},
            {label: 'Sponsor', to: '/sponsors'},
          ],
        },
        {
          title: 'Sponsors',
          items: [
            {label: 'Sponsor BackupX', href: 'https://github.com/sponsors/Awuqing'},
            {label: 'Partnership', href: 'https://github.com/Awuqing/BackupX/issues/new/choose'},
            {label: 'Sponsor tiers', to: '/sponsors'},
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
