import { Card } from '@arco-design/web-react';
import type { PropsWithChildren, ReactNode } from 'react';

interface PageCardProps extends PropsWithChildren {
  title: ReactNode;
}

export function PageCard({ title, children }: PageCardProps) {
  return (
    <Card className="page-card" title={title} bordered={false}>
      {children}
    </Card>
  );
}
