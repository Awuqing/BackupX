import type {ReactNode} from 'react';
import {useState} from 'react';
import clsx from 'clsx';
import Heading from '@theme/Heading';
import Translate from '@docusaurus/Translate';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Link from '@docusaurus/Link';
import styles from './styles.module.css';

type Tab = {
  id: string;
  label: ReactNode;
  image: string;
  title: ReactNode;
  description: ReactNode;
};

function useTabs(): Tab[] {
  return [
    {
      id: 'dashboard',
      label: <Translate id="showcase.tab.dashboard">Dashboard</Translate>,
      image: useBaseUrl('/img/screenshots/dashboard.png'),
      title: <Translate id="showcase.dashboard.title">Know at a glance</Translate>,
      description: (
        <Translate id="showcase.dashboard.desc">
          Backup success rates, storage usage, recent runs and upcoming schedules — all on one page with live data.
        </Translate>
      ),
    },
    {
      id: 'tasks',
      label: <Translate id="showcase.tab.tasks">Backup Tasks</Translate>,
      image: useBaseUrl('/img/screenshots/backup-tasks.png'),
      title: <Translate id="showcase.tasks.title">Visual task editor</Translate>,
      description: (
        <Translate id="showcase.tasks.desc">
          Files, MySQL, PostgreSQL, SQLite and SAP HANA with a three-step wizard. Cron editor, multi-target dispatch, retention, compression and encryption — point and click.
        </Translate>
      ),
    },
    {
      id: 'storage',
      label: <Translate id="showcase.tab.storage">Storage Targets</Translate>,
      image: useBaseUrl('/img/screenshots/storage-targets.png'),
      title: <Translate id="showcase.storage.title">70+ backends, one flow</Translate>,
      description: (
        <Translate id="showcase.storage.desc">
          Alibaba OSS, Tencent COS, S3, Google Drive, WebDAV — plus every rclone backend behind a uniform form. Test connection, favourite, and view live usage.
        </Translate>
      ),
    },
    {
      id: 'nodes',
      label: <Translate id="showcase.tab.nodes">Multi-Node</Translate>,
      image: useBaseUrl('/img/screenshots/nodes.png'),
      title: <Translate id="showcase.nodes.title">Master-Agent in minutes</Translate>,
      description: (
        <Translate id="showcase.nodes.desc">
          Create a node, copy the token, start the Agent on any remote host. Tasks routed to a node run locally there and upload directly to storage — no reverse connectivity required.
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
            <img src={current.image} alt="" className={styles.screenshot} />
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
