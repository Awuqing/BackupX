import type {ReactNode} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import Translate from '@docusaurus/Translate';
import styles from './styles.module.css';

type FeatureItem = {
  title: ReactNode;
  description: ReactNode;
};

const FEATURES: FeatureItem[] = [
  {
    title: <Translate id="feat.types.title">Many Backup Types</Translate>,
    description: (
      <Translate id="feat.types.desc">
        Files &amp; directories with multi-path sources, plus MySQL, PostgreSQL, SQLite, and SAP HANA — all in one place.
      </Translate>
    ),
  },
  {
    title: <Translate id="feat.storage.title">70+ Storage Backends</Translate>,
    description: (
      <Translate id="feat.storage.desc">
        Native Alibaba OSS, Tencent COS, Qiniu, S3, Google Drive, WebDAV, FTP — plus SFTP, Azure Blob, Dropbox, OneDrive and dozens more via rclone.
      </Translate>
    ),
  },
  {
    title: <Translate id="feat.scheduling.title">Scheduling &amp; Retention</Translate>,
    description: (
      <Translate id="feat.scheduling.desc">
        Cron-based schedules with a visual editor and auto-retention (by days or count), plus empty-directory cleanup.
      </Translate>
    ),
  },
  {
    title: <Translate id="feat.cluster.title">Multi-Node Cluster</Translate>,
    description: (
      <Translate id="feat.cluster.desc">
        Master-Agent mode manages backups across multiple servers. Agents run tasks locally and upload straight to storage — no reverse connectivity required.
      </Translate>
    ),
  },
  {
    title: <Translate id="feat.security.title">Secure by Default</Translate>,
    description: (
      <Translate id="feat.security.desc">
        JWT auth, bcrypt passwords, AES-256-GCM encrypted config, optional backup encryption, and a full audit log.
      </Translate>
    ),
  },
  {
    title: <Translate id="feat.deploy.title">Painless Deployment</Translate>,
    description: (
      <Translate id="feat.deploy.desc">
        Single static binary with embedded SQLite. Docker one-click or bare-metal via install.sh — zero external dependencies.
      </Translate>
    ),
  },
];

function Feature({title, description}: FeatureItem) {
  return (
    <div className={clsx('col col--4', styles.feature)}>
      <Heading as="h3">{title}</Heading>
      <p>{description}</p>
    </div>
  );
}

export default function HomepageFeatures(): ReactNode {
  return (
    <section className={styles.features}>
      <div className="container">
        <div className="row">
          {FEATURES.map((props, idx) => (
            <Feature key={idx} {...props} />
          ))}
        </div>
      </div>
    </section>
  );
}
