import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Translate from '@docusaurus/Translate';
import Heading from '@theme/Heading';
import DocIcon, {type DocIconName} from '@site/src/components/DocIcon';
import styles from './styles.module.css';

type DocumentationPath = {
  icon: DocIconName;
  title: ReactNode;
  description: ReactNode;
  to: string;
};

const DOCUMENTATION_PATHS: DocumentationPath[] = [
  {
    icon: 'box',
    title: <Translate id="feat.install.title">Install and upgrade</Translate>,
    description: <Translate id="feat.install.desc">Choose Docker, Compose, or a standalone binary and keep the deployment repeatable.</Translate>,
    to: '/docs/getting-started/installation',
  },
  {
    icon: 'database',
    title: <Translate id="feat.types.title">Protect files and databases</Translate>,
    description: <Translate id="feat.types.desc">Configure file, MySQL, PostgreSQL, SQLite, and SAP HANA backup workloads.</Translate>,
    to: '/docs/features/backup-types',
  },
  {
    icon: 'storage',
    title: <Translate id="feat.storage.title">Connect storage targets</Translate>,
    description: <Translate id="feat.storage.desc">Use native providers or any supported rclone backend through one consistent flow.</Translate>,
    to: '/docs/features/storage-backends',
  },
  {
    icon: 'network',
    title: <Translate id="feat.cluster.title">Build a remote-node cluster</Translate>,
    description: <Translate id="feat.cluster.desc">Deploy outbound-only agents through proxies, private CAs, or SSH bastion hosts.</Translate>,
    to: '/docs/features/multi-node',
  },
  {
    icon: 'monitor',
    title: <Translate id="feat.monitor.title">Monitor daily operations</Translate>,
    description: <Translate id="feat.monitor.desc">Track task health, storage capacity, notifications, logs, and service readiness.</Translate>,
    to: '/docs/operations/monitoring',
  },
  {
    icon: 'restore',
    title: <Translate id="feat.recovery.title">Recover with a tested plan</Translate>,
    description: <Translate id="feat.recovery.desc">Prepare upgrades, rollback points, restore validation, and incident troubleshooting.</Translate>,
    to: '/docs/operations/upgrade-recovery',
  },
];

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.sectionHead}>
          <div>
            <div className={styles.sectionTag}>
              <Translate id="section.features.tag">Documentation paths</Translate>
            </div>
            <Heading as="h2" className={styles.sectionTitle}>
              <Translate id="section.features.title">Move from deployment to recovery without guesswork</Translate>
            </Heading>
          </div>
          <p className={styles.sectionSubtitle}>
            <Translate id="section.features.subtitle">
              Each path connects product capability to the configuration, security, and operational decisions required in production.
            </Translate>
          </p>
        </div>

        <div className={styles.pathGrid}>
          {DOCUMENTATION_PATHS.map((path, index) => (
            <Link key={path.to} to={path.to} className={styles.pathItem}>
              <span className={styles.pathIndex}>{String(index + 1).padStart(2, '0')}</span>
              <span className={styles.pathIcon}><DocIcon name={path.icon} size={21} /></span>
              <span className={styles.pathBody}>
                <span className={styles.pathTitle}>{path.title}</span>
                <span className={styles.pathDescription}>{path.description}</span>
              </span>
              <DocIcon name="arrowRight" size={18} className={styles.pathArrow} />
            </Link>
          ))}
        </div>
      </div>
    </section>
  );
}
