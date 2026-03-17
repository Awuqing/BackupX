import {
  Alert,
  Descriptions,
  Spin,
  Tag,
  Typography,
} from '@arco-design/web-react';
import { useEffect, useState } from 'react';

import { PageCard } from '../../components/page-card';
import { systemApi } from '../../services/system';
import type { SystemInfo } from '../../types/system';

function formatUptime(seconds: number) {
  if (seconds < 60) {
    return `${seconds} 秒`;
  }

  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);

  return `${hours} 小时 ${minutes} 分钟`;
}

export function SystemInfoPage() {
  const [data, setData] = useState<SystemInfo | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let active = true;

    async function loadSystemInfo() {
      try {
        const result = await systemApi.fetchInfo();

        if (active) {
          setData(result);
          setError(null);
        }
      } catch (loadError) {
        if (active) {
          setError(loadError instanceof Error ? loadError.message : '系统信息加载失败');
        }
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    void loadSystemInfo();

    return () => {
      active = false;
    };
  }, []);

  if (loading) {
    return (
      <div className="fullscreen-center">
        <Spin tip="正在加载系统信息..." />
      </div>
    );
  }

  if (error) {
    return <Alert type="error" content={error} />;
  }

  if (!data) {
    return <Alert type="warning" content="未获取到系统信息" />;
  }

  return (
    <div className="page-stack">
      <PageCard title="系统信息">
        <Typography.Paragraph type="secondary">
          用于确认 API 服务已正常启动，并展示平台基础运行状态。
        </Typography.Paragraph>
        <Descriptions
          column={1}
          data={[
            { label: '版本', value: data.version },
            {
              label: '运行模式',
              value: <Tag color={data.mode === 'release' ? 'green' : 'arcoblue'}>{data.mode}</Tag>,
            },
            { label: '启动时间', value: data.startedAt },
            { label: '运行时长', value: formatUptime(data.uptimeSeconds) },
            { label: '数据库路径', value: data.databasePath },
          ]}
        />
      </PageCard>
    </div>
  );
}
