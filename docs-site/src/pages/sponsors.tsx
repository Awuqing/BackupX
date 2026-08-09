import type {ReactNode} from 'react';
import {translate} from '@docusaurus/Translate';
import Translate from '@docusaurus/Translate';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import {HomepageSponsors} from '@site/src/components/HomepageCommunity';
import styles from '@site/src/components/HomepageCommunity/styles.module.css';

export default function Sponsors(): ReactNode {
  return (
    <Layout
      title={translate({id: 'sponsors.pageTitle', message: 'Sponsors'})}
      description={translate({
        id: 'sponsors.pageDescription',
        message: 'Sponsor BackupX reliability, documentation, storage compatibility and long-term maintenance.',
      })}>
      <main>
        <section className={styles.section}>
          <div className="container">
            <div className={styles.sectionHead}>
              <div>
                <div className={styles.sectionTag}>
                  <Translate id="sponsors.tag">Sponsorship</Translate>
                </div>
                <Heading as="h1" className={styles.sectionTitle}>
                  <Translate id="sponsors.title">Keep critical maintenance moving</Translate>
                </Heading>
              </div>
              <p className={styles.sectionSubtitle}>
                <Translate id="sponsors.subtitle">
                  Sponsorship funds real provider validation, reliable releases, recovery drills, and better operational documentation.
                </Translate>
              </p>
            </div>
            <HomepageSponsors />
          </div>
        </section>
      </main>
    </Layout>
  );
}
