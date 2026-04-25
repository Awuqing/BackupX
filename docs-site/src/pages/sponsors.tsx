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
              <div className={styles.sectionTag}>
                <Translate id="sponsors.tag">SPONSORS</Translate>
              </div>
              <Heading as="h1" className={styles.sectionTitle}>
                <Translate id="sponsors.title">Sponsor the BackupX ecosystem</Translate>
              </Heading>
              <p className={styles.sectionSubtitle}>
                <Translate id="sponsors.subtitle">
                  Sponsorship helps keep BackupX practical for real operators: tested storage providers, reliable releases, restore confidence and better documentation.
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
