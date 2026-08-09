import type {ReactNode} from 'react';
import {translate} from '@docusaurus/Translate';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import HomepageHero from '@site/src/components/HomepageHero';
import HomepageFeatures from '@site/src/components/HomepageFeatures';
import HomepageShowcase from '@site/src/components/HomepageShowcase';
import HomepageCommunity from '@site/src/components/HomepageCommunity';

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title={translate({id: 'home.pageTitle', message: 'Backup orchestration for self-hosted servers'})}
      description={siteConfig.tagline}>
      <HomepageHero />
      <main>
        <HomepageFeatures />
        <HomepageShowcase />
        <HomepageCommunity />
      </main>
    </Layout>
  );
}
