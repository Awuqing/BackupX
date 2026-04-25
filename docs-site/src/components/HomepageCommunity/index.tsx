import type {ReactNode} from 'react';
import {useEffect, useState} from 'react';
import Heading from '@theme/Heading';
import Translate from '@docusaurus/Translate';
import Link from '@docusaurus/Link';
import styles from './styles.module.css';

type SponsorSlot = {
  brand: ReactNode;
  name: ReactNode;
  href?: string;
};

type Contributor = {
  login: string;
  avatarUrl?: string;
  contributions: number;
  type: string;
  href: string;
};

type GitHubContributor = {
  login: string;
  avatar_url?: string;
  contributions?: number;
  html_url?: string;
  type?: string;
};

type CommunityPath = {
  title: ReactNode;
  description: ReactNode;
  href: string;
};

const SPONSOR_SLOTS: SponsorSlot[] = [
  {
    brand: 'BackupX',
    name: <Translate id="community.sponsor.logo.project">Project backer</Translate>,
    href: 'https://github.com/sponsors/Awuqing',
  },
  {
    brand: 'Cloud',
    name: <Translate id="community.sponsor.logo.cloud">Cloud partner</Translate>,
  },
  {
    brand: 'Object',
    name: <Translate id="community.sponsor.logo.object">Object storage</Translate>,
  },
  {
    brand: 'CDN',
    name: <Translate id="community.sponsor.logo.cdn">CDN partner</Translate>,
  },
  {
    brand: 'DB',
    name: <Translate id="community.sponsor.logo.database">Database partner</Translate>,
  },
  {
    brand: 'Security',
    name: <Translate id="community.sponsor.logo.security">Security audit</Translate>,
  },
  {
    brand: 'Agent',
    name: <Translate id="community.sponsor.logo.agent">Remote node lab</Translate>,
  },
  {
    brand: 'Docs',
    name: <Translate id="community.sponsor.logo.docs">Docs sponsor</Translate>,
  },
  {
    brand: 'Release',
    name: <Translate id="community.sponsor.logo.release">Release sponsor</Translate>,
  },
  {
    brand: 'S3',
    name: <Translate id="community.sponsor.logo.s3">S3 compatible</Translate>,
  },
  {
    brand: 'WebDAV',
    name: <Translate id="community.sponsor.logo.webdav">WebDAV partner</Translate>,
  },
  {
    brand: 'SFTP',
    name: <Translate id="community.sponsor.logo.sftp">SFTP partner</Translate>,
  },
  {
    brand: 'Docker',
    name: <Translate id="community.sponsor.logo.docker">Container partner</Translate>,
  },
  {
    brand: 'Mirror',
    name: <Translate id="community.sponsor.logo.mirror">Mirror partner</Translate>,
  },
  {
    brand: 'Restore',
    name: <Translate id="community.sponsor.logo.restore">Restore drill</Translate>,
  },
  {
    brand: 'QA',
    name: <Translate id="community.sponsor.logo.qa">Test lab</Translate>,
  },
  {
    brand: 'OSS',
    name: <Translate id="community.sponsor.logo.oss">Open source</Translate>,
  },
  {
    brand: 'Open Slot',
    name: <Translate id="community.sponsor.logo.open">Sponsor slot open</Translate>,
  },
];

const FALLBACK_CONTRIBUTORS: Contributor[] = [
  {
    login: 'Awuqing',
    contributions: 0,
    type: 'User',
    href: 'https://github.com/Awuqing',
  },
  {
    login: 'dependabot[bot]',
    contributions: 0,
    type: 'Bot',
    href: 'https://github.com/dependabot',
  },
];

const COMMUNITY_PATHS: CommunityPath[] = [
  {
    title: <Translate id="community.path.issues.title">Report production issues</Translate>,
    description: <Translate id="community.path.issues.desc">Share logs, deployment topology and restore expectations.</Translate>,
    href: 'https://github.com/Awuqing/BackupX/issues',
  },
  {
    title: <Translate id="community.path.docs.title">Improve docs and examples</Translate>,
    description: <Translate id="community.path.docs.desc">Contribute deployment guides for storage, agents and databases.</Translate>,
    href: '/docs/development/contributing',
  },
  {
    title: <Translate id="community.path.code.title">Ship focused PRs</Translate>,
    description: <Translate id="community.path.code.desc">Keep changes small, tested and aligned with the existing architecture.</Translate>,
    href: 'https://github.com/Awuqing/BackupX/pulls',
  },
];

function SponsorLogoCard({brand, name, href}: SponsorSlot) {
  return (
    <Link className={styles.sponsorLogoTile} to={href ?? 'https://github.com/sponsors/Awuqing'}>
      <span className={styles.sponsorLogoMark}>{brand}</span>
      <span className={styles.sponsorLogoName}>{name}</span>
    </Link>
  );
}

function getInitials(login: string): string {
  return login
    .replace(/\[bot\]$/i, '')
    .split(/[-_\s]/)
    .filter(Boolean)
    .slice(0, 2)
    .map(part => part[0]?.toUpperCase())
    .join('') || login.slice(0, 2).toUpperCase();
}

function normalizeContributor(contributor: GitHubContributor): Contributor | null {
  if (!contributor.login) {
    return null;
  }
  return {
    login: contributor.login,
    avatarUrl: contributor.avatar_url,
    contributions: contributor.contributions ?? 0,
    type: contributor.type ?? 'User',
    href: contributor.html_url ?? `https://github.com/${contributor.login}`,
  };
}

function useGitHubContributors(): Contributor[] {
  const [contributors, setContributors] = useState<Contributor[]>(FALLBACK_CONTRIBUTORS);

  useEffect(() => {
    const controller = new AbortController();

    fetch('https://api.github.com/repos/Awuqing/BackupX/contributors?per_page=12', {
      signal: controller.signal,
      headers: {
        Accept: 'application/vnd.github+json',
      },
    })
      .then(response => {
        if (!response.ok) {
          throw new Error(`GitHub contributors request failed: ${response.status}`);
        }
        return response.json() as Promise<GitHubContributor[]>;
      })
      .then(payload => {
        const nextContributors = payload
          .map(normalizeContributor)
          .filter((contributor): contributor is Contributor => Boolean(contributor));

        if (nextContributors.length > 0) {
          setContributors(nextContributors);
        }
      })
      .catch(error => {
        if (error instanceof Error && error.name !== 'AbortError') {
          console.warn(error.message);
        }
      });

    return () => controller.abort();
  }, []);

  return contributors;
}

function ContributorCard({login, avatarUrl, contributions, type, href}: Contributor) {
  return (
    <Link className={styles.contributorCard} to={href}>
      {avatarUrl ? (
        <img className={styles.avatarImage} src={avatarUrl} alt="" loading="lazy" />
      ) : (
        <span className={styles.avatar} aria-hidden="true">{getInitials(login)}</span>
      )}
      <span className={styles.contributorBody}>
        <strong>{login}</strong>
        <span>
          {type === 'Bot' ? (
            <Translate id="community.contributor.botRole">Automation contributor</Translate>
          ) : (
            <Translate id="community.contributor.githubRole">GitHub contributor</Translate>
          )}
        </span>
        <em>
          <Translate id="community.contributor.contributions" values={{count: contributions}}>
            {'{count} contributions'}
          </Translate>
        </em>
      </span>
    </Link>
  );
}

export function HomepageSponsors(): ReactNode {
  return (
    <div className={styles.sponsorWall}>
      <div className={styles.sponsorWallHeader}>
        <Heading as="h3" className={styles.sponsorWallTitle}>
          <Translate id="community.sponsor.wallTitle">Sponsors</Translate>
        </Heading>
        <Link className={styles.sponsorWallAction} to="https://github.com/sponsors/Awuqing">
          <Translate id="community.sponsor.cta">Sponsor BackupX</Translate>
          <span aria-hidden="true">-&gt;</span>
        </Link>
      </div>

      <div className={styles.sponsorLogoGrid}>
        {SPONSOR_SLOTS.map((slot, index) => (
          <SponsorLogoCard key={index} {...slot} />
        ))}
      </div>
    </div>
  );
}

export default function HomepageCommunity(): ReactNode {
  const contributors = useGitHubContributors();

  return (
    <section id="community" className={styles.section}>
      <div className="container">
        <div className={styles.sectionHead}>
          <div className={styles.sectionTag}>
            <Translate id="community.tag">COMMUNITY</Translate>
          </div>
          <Heading as="h2" className={styles.sectionTitle}>
            <Translate id="community.title">Built in the open, ready for long-term operators</Translate>
          </Heading>
          <p className={styles.sectionSubtitle}>
            <Translate id="community.subtitle">
              Backup software earns trust through transparent releases, real deployment feedback and a contributor path that stays practical.
            </Translate>
          </p>
        </div>

        <HomepageSponsors />

        <div className={styles.communityGrid}>
          <div className={styles.panel}>
            <div className={styles.panelHeader}>
              <span>
                <Translate id="community.contributor.kicker">Contributors</Translate>
              </span>
              <Link to="https://github.com/Awuqing/BackupX/graphs/contributors">
                <Translate id="community.contributor.all">View all</Translate>
              </Link>
            </div>
            <div className={styles.panelNote}>
              <Translate id="community.contributor.source">Loaded from GitHub contributors API in the browser.</Translate>
            </div>
            <div className={styles.contributorList}>
              {contributors.map(contributor => (
                <ContributorCard key={contributor.login} {...contributor} />
              ))}
            </div>
          </div>

          <div className={styles.panel}>
            <div className={styles.panelHeader}>
              <span>
                <Translate id="community.path.kicker">Contributor paths</Translate>
              </span>
            </div>
            <div className={styles.pathList}>
              {COMMUNITY_PATHS.map((path, index) => (
                <Link key={index} className={styles.pathItem} to={path.href}>
                  <span className={styles.pathIndex}>{String(index + 1).padStart(2, '0')}</span>
                  <span>
                    <strong>{path.title}</strong>
                    <em>{path.description}</em>
                  </span>
                </Link>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
