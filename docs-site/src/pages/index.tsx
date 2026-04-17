import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Translate, {translate} from '@docusaurus/Translate';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import HomepageShowcase from '@site/src/components/HomepageShowcase';

import styles from './index.module.css';

function HomepageHeader() {
  return (
    <header className={styles.hero}>
      <div className={styles.heroBg} aria-hidden="true" />
      <div className={clsx('container', styles.heroInner)}>
        <div className={styles.heroContent}>
          <div className={styles.badge}>
            <span className={styles.badgeDot} />
            <Translate id="home.badge">Open-source · v1.6.0</Translate>
          </div>
          <Heading as="h1" className={styles.heroTitle}>
            <Translate id="home.title.part1">Self-hosted backup management</Translate>
            <span className={styles.heroTitleAccent}>
              <Translate id="home.title.part2">for every server.</Translate>
            </span>
          </Heading>
          <p className={styles.heroSubtitle}>
            <Translate id="home.tagline">
              One binary, one command. File / database / SAP HANA backups routed to 70+ storage backends.
            </Translate>
          </p>
          <div className={styles.actions}>
            <Link className={clsx('button button--primary button--lg', styles.primaryBtn)} to="/docs/getting-started/quick-start">
              <Translate id="home.getStarted">Get Started</Translate>
              <span className={styles.btnArrow} aria-hidden="true">→</span>
            </Link>
            <Link className={clsx('button button--lg', styles.secondaryBtn)} to="https://github.com/Awuqing/BackupX">
              <svg width="18" height="18" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" style={{marginRight: 6}}>
                <path d="M8 0C3.58 0 0 3.58 0 8a8 8 0 005.47 7.59c.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
              </svg>
              GitHub
            </Link>
          </div>
          <div className={styles.metrics}>
            <MetricItem labelId="home.metric.backends" valueClass={styles.metricValue}>70+</MetricItem>
            <div className={styles.metricDivider} />
            <MetricItem labelId="home.metric.backupTypes" valueClass={styles.metricValue}>5</MetricItem>
            <div className={styles.metricDivider} />
            <MetricItem labelId="home.metric.license" valueClass={styles.metricValue}>Apache 2.0</MetricItem>
          </div>
        </div>
        <div className={styles.heroCode}>
          <div className={styles.codeWindow}>
            <div className={styles.codeHeader}>
              <span className={clsx(styles.codeDot, styles.codeDotRed)} />
              <span className={clsx(styles.codeDot, styles.codeDotYellow)} />
              <span className={clsx(styles.codeDot, styles.codeDotGreen)} />
              <span className={styles.codeTitle}>bash</span>
            </div>
            <pre className={styles.codeBody}>
              <code>
                <span className={styles.codeComment}># Docker one-liner</span>{'\n'}
                <span className={styles.codePrompt}>$</span> docker run -d --name backupx \{'\n'}
                {'    '}-p 8340:8340 \{'\n'}
                {'    '}-v backupx-data:/app/data \{'\n'}
                {'    '}awuqing/backupx:latest{'\n'}
                {'\n'}
                <span className={styles.codeComment}># Open http://localhost:8340</span>{'\n'}
                <span className={styles.codeComment}># Deploy an Agent on a remote host</span>{'\n'}
                <span className={styles.codePrompt}>$</span> backupx agent \{'\n'}
                {'    '}--master <span className={styles.codeString}>http://master:8340</span> \{'\n'}
                {'    '}--token <span className={styles.codeString}>&lt;token&gt;</span>
              </code>
            </pre>
          </div>
        </div>
      </div>
    </header>
  );
}

function MetricItem({children, labelId, valueClass}: {children: ReactNode; labelId: string; valueClass: string}) {
  return (
    <div className={styles.metric}>
      <div className={valueClass}>{children}</div>
      <div className={styles.metricLabel}>
        <Translate id={labelId}>
          {labelId === 'home.metric.backends' ? 'Storage backends'
            : labelId === 'home.metric.backupTypes' ? 'Backup types'
            : 'License'}
        </Translate>
      </div>
    </div>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={translate({id: 'home.pageTitle', message: 'Self-hosted backup management'})}
      description={siteConfig.tagline}>
      <HomepageHeader />
      <main>
        <HomepageFeatures />
        <HomepageShowcase />
      </main>
    </Layout>
  );
}
