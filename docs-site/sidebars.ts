import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docs: [
    'intro',
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/installation',
        'getting-started/quick-start',
      ],
    },
    {
      type: 'category',
      label: 'Deployment',
      items: [
        'deployment/docker',
        'deployment/bare-metal',
        'deployment/nginx',
        'deployment/configuration',
      ],
    },
    {
      type: 'category',
      label: 'Features',
      items: [
        'features/backup-types',
        'features/storage-backends',
        'features/sap-hana',
        'features/multi-node',
        'features/notifications',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/api',
        'reference/cli',
      ],
    },
    {
      type: 'category',
      label: 'Development',
      items: [
        'development/setup',
        'development/contributing',
      ],
    },
  ],
};

export default sidebars;
