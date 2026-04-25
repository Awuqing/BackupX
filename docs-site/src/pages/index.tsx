import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Translate, {translate} from '@docusaurus/Translate';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import HomepageShowcase from '@site/src/components/HomepageShowcase';
import HomepageCommunity from '@site/src/components/HomepageCommunity';

import styles from './index.module.css';

function HomepageHeader() {
  return (
    <header className={styles.hero}>
      <div className={clsx('container', styles.heroInner)}>
        <div className={styles.heroContent}>
          <div className={styles.badge}>
            <span className={styles.badgeDot} />
            <Translate id="home.badge">Open-source backup control plane · v2.2.1</Translate>
          </div>
          <Heading as="h1" className={styles.heroTitle}>
            <Translate id="home.title.part1">Backup orchestration</Translate>
            <span className={styles.heroTitleAccent}>
              <Translate id="home.title.part2">for self-hosted servers.</Translate>
            </span>
          </Heading>
          <p className={styles.heroSubtitle}>
            <Translate id="home.tagline">
              Run file, database, SAP HANA and remote-node backups from one clean console. Keep the control plane yours, keep the storage flexible.
            </Translate>
          </p>
          <div className={styles.actions}>
            <Link className={clsx('button button--primary button--lg', styles.primaryBtn)} to="/docs/getting-started/quick-start">
              <Translate id="home.getStarted">Get Started</Translate>
              <span className={styles.btnArrow} aria-hidden="true">-&gt;</span>
            </Link>
            <Link className={clsx('button button--lg', styles.secondaryBtn)} to="https://github.com/Awuqing/BackupX">
              <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" style={{marginRight: 6}}>
                <path d="M8 0C3.58 0 0 3.58 0 8a8 8 0 005.47 7.59c.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
              </svg>
              GitHub
            </Link>
          </div>
          <div className={styles.metrics}>
            <div className={styles.metric}>
              <div className={styles.metricValue}>70+</div>
              <div className={styles.metricLabel}>
                <Translate id="home.metric.backends">Storage backends</Translate>
              </div>
            </div>
            <div className={styles.metricDivider} />
            <div className={styles.metric}>
              <div className={styles.metricValue}>Agent</div>
              <div className={styles.metricLabel}>
                <Translate id="home.metric.backupTypes">Remote execution</Translate>
              </div>
            </div>
            <div className={styles.metricDivider} />
            <div className={styles.metric}>
              <div className={styles.metricValue}>Apache 2.0</div>
              <div className={styles.metricLabel}>
                <Translate id="home.metric.license">License</Translate>
              </div>
            </div>
          </div>
        </div>
        <div className={styles.heroVisual}>
          <div className={styles.consolePanel}>
            <div className={styles.consoleHeader}>
              <div>
                <span className={styles.consoleEyebrow}>
                  <Translate id="home.visual.eyebrow">BackupX Console</Translate>
                </span>
                <strong>
                  <Translate id="home.visual.title">Operations overview</Translate>
                </strong>
              </div>
              <span className={styles.consoleStatus}>
                <Translate id="home.visual.status">Healthy</Translate>
              </span>
            </div>
            <div className={styles.consoleGrid}>
              <div>
                <span className={styles.consoleLabel}>
                  <Translate id="home.visual.success">Success rate</Translate>
                </span>
                <strong>99.4%</strong>
              </div>
              <div>
                <span className={styles.consoleLabel}>
                  <Translate id="home.visual.nodes">Active nodes</Translate>
                </span>
                <strong>12</strong>
              </div>
              <div>
                <span className={styles.consoleLabel}>
                  <Translate id="home.visual.targets">Storage targets</Translate>
                </span>
                <strong>8</strong>
              </div>
            </div>
            <div className={styles.timeline}>
              <div className={styles.timelineRow}>
                <span className={styles.timelineDotOk} />
                <div>
                  <strong>
                    <Translate id="home.visual.row1.title">PostgreSQL nightly</Translate>
                  </strong>
                  <span>
                    <Translate id="home.visual.row1.desc">Encrypted archive uploaded to S3</Translate>
                  </span>
                </div>
                <em>02:10</em>
              </div>
              <div className={styles.timelineRow}>
                <span className={styles.timelineDotInfo} />
                <div>
                  <strong>
                    <Translate id="home.visual.row2.title">SAP HANA snapshot</Translate>
                  </strong>
                  <span>
                    <Translate id="home.visual.row2.desc">Running on agent-shanghai-02</Translate>
                  </span>
                </div>
                <em>68%</em>
              </div>
              <div className={styles.timelineRow}>
                <span className={styles.timelineDotWarn} />
                <div>
                  <strong>
                    <Translate id="home.visual.row3.title">Retention cleanup</Translate>
                  </strong>
                  <span>
                    <Translate id="home.visual.row3.desc">Next run in 4 hours</Translate>
                  </span>
                </div>
                <em>queued</em>
              </div>
            </div>
          </div>
          <div className={styles.commandCard}>
            <div className={styles.commandTitle}>
              <Translate id="home.command.title">Start with Docker</Translate>
            </div>
            <code>docker run -d -p 8340:8340 awuqing/backupx:v2.2.1</code>
          </div>
        </div>
      </div>
    </header>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={translate({id: 'home.pageTitle', message: 'Backup orchestration for self-hosted servers'})}
      description={siteConfig.tagline}>
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <HomepageShowcase />
        <HomepageCommunity />
      </main>
    </Layout>
  );
}
