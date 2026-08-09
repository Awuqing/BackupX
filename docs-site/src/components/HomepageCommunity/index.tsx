import type {ReactNode} from 'react';
import {useEffect, useState} from 'react';
import Heading from '@theme/Heading';
import Translate from '@docusaurus/Translate';
import Link from '@docusaurus/Link';
import DocIcon, {type DocIconName} from '@site/src/components/DocIcon';
import styles from './styles.module.css';

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
  icon: DocIconName;
  title: ReactNode;
  description: ReactNode;
  href: string;
};

type SponsorFocus = {
  id: string;
  icon: DocIconName;
  label: ReactNode;
  title: ReactNode;
  description: ReactNode;
};

type SponsorTier = {
  id: string;
  name: ReactNode;
  audience: ReactNode;
  description: ReactNode;
};

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
    icon: 'wrench',
    title: <Translate id="community.path.issues.title">Report production issues</Translate>,
    description: <Translate id="community.path.issues.desc">Share logs, deployment topology, and restore expectations.</Translate>,
    href: 'https://github.com/Awuqing/BackupX/issues',
  },
  {
    icon: 'bookOpen',
    title: <Translate id="community.path.docs.title">Improve docs and examples</Translate>,
    description: <Translate id="community.path.docs.desc">Contribute deployment guides for storage, agents, and databases.</Translate>,
    href: '/docs/development/contributing',
  },
  {
    icon: 'github',
    title: <Translate id="community.path.code.title">Ship focused pull requests</Translate>,
    description: <Translate id="community.path.code.desc">Keep changes tested, reviewable, and aligned with the existing architecture.</Translate>,
    href: 'https://github.com/Awuqing/BackupX/pulls',
  },
];

const SPONSOR_FOCUS: SponsorFocus[] = [
  {
    id: 'infrastructure',
    icon: 'cloud',
    label: <Translate id="community.sponsor.infrastructure.label">Infrastructure</Translate>,
    title: <Translate id="community.sponsor.infrastructure.title">Cloud and storage compatibility</Translate>,
    description: <Translate id="community.sponsor.infrastructure.desc">Support validation across object storage, WebDAV, SFTP, and regional cloud providers.</Translate>,
  },
  {
    id: 'reliability',
    icon: 'shield',
    label: <Translate id="community.sponsor.security.label">Reliability</Translate>,
    title: <Translate id="community.sponsor.security.title">Security and recovery work</Translate>,
    description: <Translate id="community.sponsor.security.desc">Fund encryption reviews, restore drills, release signing, and operational checks.</Translate>,
  },
  {
    id: 'community',
    icon: 'users',
    label: <Translate id="community.sponsor.community.label">Community</Translate>,
    title: <Translate id="community.sponsor.community.title">Documentation and contributor support</Translate>,
    description: <Translate id="community.sponsor.community.desc">Improve guides, examples, platform testing, and the contributor experience.</Translate>,
  },
];

const SPONSOR_TIERS: SponsorTier[] = [
  {
    id: 'backer',
    name: <Translate id="community.sponsor.tier.backer.name">Backer</Translate>,
    audience: <Translate id="community.sponsor.tier.backer.amount">For individuals and small teams</Translate>,
    description: <Translate id="community.sponsor.tier.backer.desc">Supports documentation, issue triage, compatibility testing, and focused usability work.</Translate>,
  },
  {
    id: 'partner',
    name: <Translate id="community.sponsor.tier.partner.name">Partner</Translate>,
    audience: <Translate id="community.sponsor.tier.partner.amount">For storage and infrastructure vendors</Translate>,
    description: <Translate id="community.sponsor.tier.partner.desc">Supports provider validation, deployment examples, benchmarks, and integration guides.</Translate>,
  },
  {
    id: 'enterprise',
    name: <Translate id="community.sponsor.tier.enterprise.name">Enterprise</Translate>,
    audience: <Translate id="community.sponsor.tier.enterprise.amount">For production BackupX operators</Translate>,
    description: <Translate id="community.sponsor.tier.enterprise.desc">Funds recovery drills, release hardening, audits, and long-term maintenance.</Translate>,
  },
];

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

    fetch('https://api.github.com/repos/Awuqing/BackupX/contributors?per_page=8', {
      signal: controller.signal,
      headers: {Accept: 'application/vnd.github+json'},
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

function ContributorRow({login, avatarUrl, contributions, type, href}: Contributor): ReactNode {
  return (
    <Link className={styles.contributorRow} to={href}>
      {avatarUrl ? (
        <img className={styles.avatarImage} src={avatarUrl} alt="" loading="lazy" />
      ) : (
        <span className={styles.avatar} aria-hidden="true">{getInitials(login)}</span>
      )}
      <span className={styles.contributorBody}>
        <span className={styles.contributorName}>{login}</span>
        <span className={styles.contributorRole}>
          {type === 'Bot' ? (
            <Translate id="community.contributor.botRole">Automation contributor</Translate>
          ) : (
            <Translate id="community.contributor.githubRole">GitHub contributor</Translate>
          )}
        </span>
      </span>
      <span className={styles.contributionCount}>
        <Translate id="community.contributor.contributions" values={{count: contributions}}>
          {'{count} contributions'}
        </Translate>
      </span>
    </Link>
  );
}

export function HomepageSponsors(): ReactNode {
  return (
    <div className={styles.sponsorProgram}>
      <div className={styles.sponsorProgramHeader}>
        <span className={styles.sponsorProgramIcon}><DocIcon name="heart" size={23} /></span>
        <div className={styles.sponsorProgramCopy}>
          <Heading as="h2" className={styles.sponsorProgramTitle}>
            <Translate id="community.sponsor.title">Support reliable backup infrastructure</Translate>
          </Heading>
          <p>
            <Translate id="community.sponsor.programDesc">
              Sponsorship is directed toward test coverage, restore confidence, provider compatibility, and documentation that operators can apply directly.
            </Translate>
          </p>
        </div>
        <Link className={styles.sponsorAction} to="https://github.com/sponsors/Awuqing">
          <DocIcon name="github" size={17} />
          <Translate id="community.sponsor.cta">Sponsor BackupX</Translate>
          <DocIcon name="external" size={15} />
        </Link>
      </div>

      <div className={styles.sponsorFocusList}>
        {SPONSOR_FOCUS.map(focus => (
          <div key={focus.id} className={styles.sponsorFocusItem}>
            <span className={styles.sponsorFocusIcon}><DocIcon name={focus.icon} size={20} /></span>
            <span className={styles.sponsorFocusBody}>
              <span className={styles.sponsorFocusLabel}>{focus.label}</span>
              <span className={styles.sponsorFocusTitle}>{focus.title}</span>
              <span className={styles.sponsorFocusDescription}>{focus.description}</span>
            </span>
          </div>
        ))}
      </div>

      <div className={styles.tierSection}>
        <div className={styles.tierSectionHeader}>
          <Heading as="h3"><Translate id="community.sponsor.tier.title">Ways to support</Translate></Heading>
          <p><Translate id="community.sponsor.tier.subtitle">Choose a level that matches how your team depends on BackupX.</Translate></p>
        </div>
        <div className={styles.tierGrid}>
          {SPONSOR_TIERS.map(tier => (
            <div key={tier.id} className={styles.tierItem}>
              <span className={styles.tierName}>{tier.name}</span>
              <span className={styles.tierAudience}>{tier.audience}</span>
              <span className={styles.tierDescription}>{tier.description}</span>
            </div>
          ))}
        </div>
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
          <div>
            <div className={styles.sectionTag}>
              <Translate id="community.tag">Community</Translate>
            </div>
            <Heading as="h2" className={styles.sectionTitle}>
              <Translate id="community.title">Operational knowledge improves in the open</Translate>
            </Heading>
          </div>
          <p className={styles.sectionSubtitle}>
            <Translate id="community.subtitle">
              Report real deployment constraints, improve the runbooks, or contribute a focused change with reproducible validation.
            </Translate>
          </p>
        </div>

        <div className={styles.communityShell}>
          <div className={styles.pathPanel}>
            <div className={styles.panelHeader}>
              <span><Translate id="community.path.kicker">Contribution paths</Translate></span>
              <Link to="/docs/development/contributing">
                <Translate id="community.path.guide">Contribution guide</Translate>
                <DocIcon name="arrowRight" size={15} />
              </Link>
            </div>
            <div className={styles.pathList}>
              {COMMUNITY_PATHS.map((path, index) => (
                <Link key={path.href} className={styles.pathItem} to={path.href}>
                  <span className={styles.pathIndex}>{String(index + 1).padStart(2, '0')}</span>
                  <span className={styles.pathIcon}><DocIcon name={path.icon} size={19} /></span>
                  <span className={styles.pathBody}>
                    <span className={styles.pathTitle}>{path.title}</span>
                    <span className={styles.pathDescription}>{path.description}</span>
                  </span>
                  <DocIcon name="arrowRight" size={17} className={styles.rowArrow} />
                </Link>
              ))}
            </div>
          </div>

          <div className={styles.contributorPanel}>
            <div className={styles.panelHeader}>
              <span><Translate id="community.contributor.kicker">Contributors</Translate></span>
              <Link to="https://github.com/Awuqing/BackupX/graphs/contributors">
                <Translate id="community.contributor.all">View all</Translate>
                <DocIcon name="external" size={14} />
              </Link>
            </div>
            <p className={styles.panelNote}>
              <Translate id="community.contributor.source">Loaded from the GitHub contributors API with a local fallback.</Translate>
            </p>
            <div className={styles.contributorList}>
              {contributors.slice(0, 5).map(contributor => (
                <ContributorRow key={contributor.login} {...contributor} />
              ))}
            </div>
          </div>

          <div className={styles.sponsorBand}>
            <span className={styles.sponsorBandIcon}><DocIcon name="heart" size={20} /></span>
            <span className={styles.sponsorBandBody}>
              <span className={styles.sponsorBandTitle}><Translate id="community.sponsor.bandTitle">Support long-term maintenance</Translate></span>
              <span className={styles.sponsorBandDescription}><Translate id="community.sponsor.bandDesc">Fund compatibility testing, recovery work, and operator-focused documentation.</Translate></span>
            </span>
            <Link to="/sponsors">
              <Translate id="community.sponsor.learnMore">Sponsorship details</Translate>
              <DocIcon name="arrowRight" size={16} />
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}
