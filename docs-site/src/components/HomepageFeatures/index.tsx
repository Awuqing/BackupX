import type {ReactNode} from 'react';
import Heading from '@theme/Heading';
import Translate from '@docusaurus/Translate';
import Link from '@docusaurus/Link';
import styles from './styles.module.css';

type FeatureItem = {
  title: ReactNode;
  description: ReactNode;
  icon: ReactNode;
  link?: string;
};

const DatabaseIcon = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <ellipse cx="12" cy="5" rx="9" ry="3" />
    <path d="M3 5v6c0 1.66 4 3 9 3s9-1.34 9-3V5" />
    <path d="M3 11v6c0 1.66 4 3 9 3s9-1.34 9-3v-6" />
  </svg>
);

const CloudIcon = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M18 10h-1.26A8 8 0 109 20h9a5 5 0 000-10z" />
  </svg>
);

const ClockIcon = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <circle cx="12" cy="12" r="10" />
    <polyline points="12 6 12 12 16 14" />
  </svg>
);

const NetworkIcon = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <rect x="9" y="2" width="6" height="6" rx="1" />
    <rect x="2" y="16" width="6" height="6" rx="1" />
    <rect x="16" y="16" width="6" height="6" rx="1" />
    <path d="M12 8v4" />
    <path d="M12 12H5v4" />
    <path d="M12 12h7v4" />
  </svg>
);

const ShieldIcon = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M12 2l9 4v6c0 5-3.5 9.5-9 10-5.5-.5-9-5-9-10V6l9-4z" />
    <polyline points="9 12 11 14 15 10" />
  </svg>
);

const RocketIcon = () => (
  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
    <path d="M4.5 16.5c-1.5 1.26-2 5-2 5s3.74-.5 5-2c.71-.84.7-2.13-.09-2.91a2.18 2.18 0 00-2.91-.09z" />
    <path d="M12 15l-3-3a22 22 0 012-3.95A12.88 12.88 0 0122 2c0 2.72-.78 7.5-6 11a22.35 22.35 0 01-4 2z" />
    <path d="M9 12H4s.55-3.03 2-4c1.62-1.08 5 0 5 0" />
    <path d="M12 15v5s3.03-.55 4-2c1.08-1.62 0-5 0-5" />
  </svg>
);

const FEATURES: FeatureItem[] = [
  {
    title: <Translate id="feat.types.title">Many Backup Types</Translate>,
    description: (
      <Translate id="feat.types.desc">
        Files and directories with multi-path sources, plus MySQL, PostgreSQL, SQLite, and SAP HANA — all in one place.
      </Translate>
    ),
    icon: <DatabaseIcon />,
    link: '/docs/features/backup-types',
  },
  {
    title: <Translate id="feat.storage.title">70+ Storage Backends</Translate>,
    description: (
      <Translate id="feat.storage.desc">
        Native Alibaba OSS, Tencent COS, Qiniu, S3, Google Drive, WebDAV, FTP — plus SFTP, Azure Blob, Dropbox and more via rclone.
      </Translate>
    ),
    icon: <CloudIcon />,
    link: '/docs/features/storage-backends',
  },
  {
    title: <Translate id="feat.scheduling.title">Scheduling & Retention</Translate>,
    description: (
      <Translate id="feat.scheduling.desc">
        Cron-based schedules with a visual editor and auto-retention (by days or count), plus empty-directory cleanup.
      </Translate>
    ),
    icon: <ClockIcon />,
  },
  {
    title: <Translate id="feat.cluster.title">Multi-Node Cluster</Translate>,
    description: (
      <Translate id="feat.cluster.desc">
        Master-Agent via HTTP long-polling. Agents run tasks locally and upload directly to storage — no reverse connectivity.
      </Translate>
    ),
    icon: <NetworkIcon />,
    link: '/docs/features/multi-node',
  },
  {
    title: <Translate id="feat.security.title">Secure by Default</Translate>,
    description: (
      <Translate id="feat.security.desc">
        JWT auth, bcrypt passwords, AES-256-GCM encrypted config, optional backup encryption, and a full audit log.
      </Translate>
    ),
    icon: <ShieldIcon />,
  },
  {
    title: <Translate id="feat.deploy.title">Painless Deployment</Translate>,
    description: (
      <Translate id="feat.deploy.desc">
        Single static binary with embedded SQLite. Docker one-click or bare-metal — zero external dependencies.
      </Translate>
    ),
    icon: <RocketIcon />,
    link: '/docs/getting-started/installation',
  },
];

function Feature({title, description, icon, link}: FeatureItem) {
  const content = (
    <>
      <div className={styles.iconWrap}>{icon}</div>
      <Heading as="h3" className={styles.featureTitle}>{title}</Heading>
      <p className={styles.featureDesc}>{description}</p>
      {link && (
        <span className={styles.featureLink}>
          <Translate id="feat.learnMore">Learn more</Translate>
          <span className={styles.featureArrow} aria-hidden="true">-&gt;</span>
        </span>
      )}
    </>
  );
  if (link) {
    return (
      <Link to={link} className={styles.featureCardLink}>
        {content}
      </Link>
    );
  }
  return <div className={styles.featureCard}>{content}</div>;
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.sectionHead}>
          <div className={styles.sectionTag}>
            <Translate id="section.features.tag">FEATURES</Translate>
          </div>
          <Heading as="h2" className={styles.sectionTitle}>
            <Translate id="section.features.title">Everything you need, nothing you don't</Translate>
          </Heading>
          <p className={styles.sectionSubtitle}>
            <Translate id="section.features.subtitle">
              Battle-tested building blocks — backup runners, storage providers, scheduling, and clustering.
            </Translate>
          </p>
        </div>
        <div className={styles.grid}>
          {FEATURES.map((feat, idx) => (
            <Feature key={idx} {...feat} />
          ))}
        </div>
      </div>
    </section>
  );
}
