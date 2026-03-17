import { Grid, Statistic, Typography } from '@arco-design/web-react';

import { PageCard } from '../../components/page-card';

const cards = [
  { label: '存储目标', value: 0 },
  { label: '备份任务', value: 0 },
  { label: '最近执行', value: 0 },
];

export function DashboardPage() {
  return (
    <div className="page-stack">
      <PageCard title="平台概览">
        <Typography.Paragraph type="secondary">
          `platform-foundation` 阶段提供基础登录、导航与系统状态展示，后续模块将在此页面扩展统计与运行数据。
        </Typography.Paragraph>
      </PageCard>
      <Grid.Row gutter={16}>
        {cards.map((card) => (
          <Grid.Col key={card.label} xs={24} md={8}>
            <PageCard title={card.label}>
              <Statistic title={card.label} value={card.value} />
            </PageCard>
          </Grid.Col>
        ))}
      </Grid.Row>
    </div>
  );
}
