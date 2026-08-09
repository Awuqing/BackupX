import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import Translate, {translate} from '@docusaurus/Translate';
import Heading from '@theme/Heading';
import DocIcon, {type DocIconName} from '@site/src/components/DocIcon';
import styles from './styles.module.css';

type GuideLink = {
  icon: DocIconName;
  title: ReactNode;
  description: ReactNode;
  to: string;
};

const GUIDE_LINKS: GuideLink[] = [
  {
    icon: 'box',
    title: <Translate id="home.guide.install.title">Install BackupX</Translate>,
    description: <Translate id="home.guide.install.desc">Docker, Compose, or a standalone binary</Translate>,
    to: '/docs/getting-started/installation',
  },
  {
    icon: 'network',
    title: <Translate id="home.guide.cluster.title">Connect remote nodes</Translate>,
    description: <Translate id="home.guide.cluster.desc">Agents, proxies, private CAs, and bastion hosts</Translate>,
    to: '/docs/features/multi-node',
  },
  {
    icon: 'shield',
    title: <Translate id="home.guide.security.title">Harden operations</Translate>,
    description: <Translate id="home.guide.security.desc">Security controls, monitoring, and audit trails</Translate>,
    to: '/docs/operations/security',
  },
  {
    icon: 'restore',
    title: <Translate id="home.guide.recovery.title">Prepare recovery</Translate>,
    description: <Translate id="home.guide.recovery.desc">Upgrade, rollback, restore, and troubleshoot</Translate>,
    to: '/docs/operations/upgrade-recovery',
  },
];

export default function HomepageHero(): ReactNode {
  return (
    <header className={styles.hero}>
      <div className={`container ${styles.heroGrid}`}>
        <div className={styles.heroContent}>
          <div className={styles.badge}>
            <DocIcon name="bookOpen" size={16} />
            <Translate id="home.badge">BackupX documentation · v2.2.1</Translate>
          </div>

          <Heading as="h1" className={styles.heroTitle}>
            <span className={styles.heroTitleLine}><Translate id="home.title.part1">Operate BackupX</Translate></span>
            <span className={styles.heroTitleAccent}><Translate id="home.title.part2">with confidence.</Translate></span>
          </Heading>

          <p className={styles.heroSubtitle}>
            <Translate id="home.tagline">
              Deploy the control plane, connect storage and remote agents, then keep backups observable and recoverable with one practical guide set.
            </Translate>
          </p>

          <div className={styles.actions}>
            <Link className={styles.primaryAction} to="/docs/getting-started/quick-start">
              <DocIcon name="terminal" size={18} />
              <Translate id="home.getStarted">Start with Docker</Translate>
              <DocIcon name="arrowRight" size={17} />
            </Link>
            <Link className={styles.secondaryAction} to="https://github.com/Awuqing/BackupX">
              <DocIcon name="github" size={18} />
              <Translate id="home.viewSource">View source</Translate>
            </Link>
          </div>

          <div className={styles.supported} aria-label={translate({id: 'home.supported.label', message: 'Supported environments'})}>
            <span><DocIcon name="check" size={16} /><Translate id="home.supported.docker">Docker</Translate></span>
            <span><DocIcon name="check" size={16} /><Translate id="home.supported.linux">Linux</Translate></span>
            <span><DocIcon name="check" size={16} /><Translate id="home.supported.windows">Windows agents</Translate></span>
          </div>
        </div>

        <nav className={styles.guidePanel} aria-label={translate({id: 'home.guide.label', message: 'Recommended documentation paths'})}>
          <div className={styles.guidePanelHeader}>
            <span><Translate id="home.guide.kicker">Start here</Translate></span>
            <span><Translate id="home.guide.hint">Choose a guide for the next task</Translate></span>
          </div>
          <div className={styles.guideList}>
            {GUIDE_LINKS.map(guide => (
              <Link key={guide.to} className={styles.guideLink} to={guide.to}>
                <span className={styles.guideIcon}><DocIcon name={guide.icon} size={20} /></span>
                <span className={styles.guideBody}>
                  <span className={styles.guideTitle}>{guide.title}</span>
                  <span className={styles.guideDescription}>{guide.description}</span>
                </span>
                <DocIcon name="arrowRight" size={17} className={styles.guideArrow} />
              </Link>
            ))}
          </div>
        </nav>
      </div>
    </header>
  );
}
