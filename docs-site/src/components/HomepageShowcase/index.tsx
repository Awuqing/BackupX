import type {ReactNode} from 'react';
import {useState} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import Translate, {translate} from '@docusaurus/Translate';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Link from '@docusaurus/Link';
import styles from './styles.module.css';

type Tab = {
  id: string;
  label: ReactNode;
  image: string;
  imageAlt: string;
  title: ReactNode;
  description: ReactNode;
};

function useTabs(): Tab[] {
  return [
    {
      id: 'dashboard',
      label: <Translate id="showcase.tab.dashboard">Dashboard</Translate>,
      image: useBaseUrl('/img/screenshots/dashboard.png'),
      imageAlt: translate({
        id: 'showcase.dashboard.alt',
        message: 'BackupX dashboard showing 30-day backup trends, storage distribution, task volume and cluster health',
      }),
      title: <Translate id="showcase.dashboard.title">Operations at a glance</Translate>,
      description: (
        <Translate id="showcase.dashboard.desc">
          Track 30-day success and failure trends, storage distribution, task counts, data volume and recent runs from one live operations view.
        </Translate>
      ),
    },
    {
      id: 'tasks',
      label: <Translate id="showcase.tab.tasks">Backup Tasks</Translate>,
      image: useBaseUrl('/img/screenshots/backup-tasks.png'),
      imageAlt: translate({
        id: 'showcase.tasks.alt',
        message: 'BackupX task list showing schedules, storage targets, retention, tags, RPO goals and verification status',
      }),
      title: <Translate id="showcase.tasks.title">Policies you can scan</Translate>,
      description: (
        <Translate id="showcase.tasks.desc">
          Review schedules, multi-target policies, retention, tags, RPO goals and verification status together, then act on any task in one click.
        </Translate>
      ),
    },
    {
      id: 'storage',
      label: <Translate id="showcase.tab.storage">Storage Targets</Translate>,
      image: useBaseUrl('/img/screenshots/storage-targets.png'),
      imageAlt: translate({
        id: 'showcase.storage.alt',
        message: 'BackupX storage targets showing connection health, live capacity, favourites and redundancy roles',
      }),
      title: <Translate id="showcase.storage.title">Every target, one view</Translate>,
      description: (
        <Translate id="showcase.storage.desc">
          Compare connection health, live capacity, favourites and redundancy roles across local disks and 70+ remote backends.
        </Translate>
      ),
    },
    {
      id: 'nodes',
      label: <Translate id="showcase.tab.nodes">Multi-Node</Translate>,
      image: useBaseUrl('/img/screenshots/nodes.png'),
      imageAlt: translate({
        id: 'showcase.nodes.alt',
        message: 'BackupX node list showing health, Agent versions, queue depth, labels and heartbeat times',
      }),
      title: <Translate id="showcase.nodes.title">Cluster health in one view</Translate>,
      description: (
        <Translate id="showcase.nodes.desc">
          Monitor health, Agent versions, queue depth, labels and heartbeat time across the local Master and every remote node.
        </Translate>
      ),
    },
  ];
}

export default function HomepageShowcase(): ReactNode {
  const tabs = useTabs();
  const [active, setActive] = useState(tabs[0].id);
  const current = tabs.find(t => t.id === active) ?? tabs[0];
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.sectionHead}>
          <div className={styles.sectionTag}>
            <Translate id="showcase.tag">PRODUCT</Translate>
          </div>
          <Heading as="h2" className={styles.sectionTitle}>
            <Translate id="showcase.title">A polished console, not a DIY script</Translate>
          </Heading>
          <p className={styles.sectionSubtitle}>
            <Translate id="showcase.subtitle">
              Every screen designed for day-2 operations — visibility first, configuration second.
            </Translate>
          </p>
        </div>
        <div className={styles.tabs}>
          {tabs.map(tab => (
            <button
              key={tab.id}
              type="button"
              className={clsx(styles.tabBtn, active === tab.id && styles.tabBtnActive)}
              onClick={() => setActive(tab.id)}>
              {tab.label}
            </button>
          ))}
        </div>
        <div className={styles.stage}>
          <div className={styles.browser}>
            <div className={styles.browserBar}>
              <span className={clsx(styles.browserDot, styles.browserDotRed)} />
              <span className={clsx(styles.browserDot, styles.browserDotYellow)} />
              <span className={clsx(styles.browserDot, styles.browserDotGreen)} />
              <div className={styles.browserUrl}>backupx.local</div>
            </div>
            <img src={current.image} alt={current.imageAlt} className={styles.screenshot} />
          </div>
          <div className={styles.caption}>
            <Heading as="h3" className={styles.captionTitle}>{current.title}</Heading>
            <p className={styles.captionDesc}>{current.description}</p>
            <Link to="/docs/getting-started/quick-start" className={styles.captionLink}>
              <Translate id="showcase.cta">Explore the docs</Translate>
              <span aria-hidden="true"> -&gt;</span>
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}
