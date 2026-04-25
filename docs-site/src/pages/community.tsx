import type {ReactNode} from 'react';
import {translate} from '@docusaurus/Translate';
import Layout from '@theme/Layout';
import HomepageCommunity from '@site/src/components/HomepageCommunity';

export default function Community(): ReactNode {
  return (
    <Layout
      title={translate({id: 'community.pageTitle', message: 'Community, sponsors and contributors'})}
      description={translate({
        id: 'community.pageDescription',
        message: 'Sponsor BackupX, meet contributors, and find practical ways to contribute.',
      })}>
      <main>
        <HomepageCommunity />
      </main>
    </Layout>
  );
}
