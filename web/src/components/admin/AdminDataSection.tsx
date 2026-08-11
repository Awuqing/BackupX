import { Typography } from '@arco-design/web-react'
import type { ReactNode } from 'react'

export interface AdminMetric {
  label: string
  value: ReactNode
  detail: string
}

interface AdminDataSectionProps {
  title: string
  description: string
  actions?: ReactNode
  metrics: AdminMetric[]
  toolbar: ReactNode
  children: ReactNode
}

export function AdminDataSection({ title, description, actions, metrics, toolbar, children }: AdminDataSectionProps) {
  return (
    <section className="admin-section" aria-labelledby="admin-section-title">
      <header className="admin-section__header">
        <div>
          <Typography.Title id="admin-section-title" heading={5} className="admin-section__title">
            {title}
          </Typography.Title>
          <Typography.Paragraph type="secondary" className="admin-section__description">
            {description}
          </Typography.Paragraph>
        </div>
        {actions}
      </header>

      <div className="admin-summary" aria-label={`${title}概览`}>
        {metrics.map((metric) => (
          <div key={metric.label} className="admin-summary__item">
            <Typography.Text type="secondary">{metric.label}</Typography.Text>
            <span className="admin-summary__value">{metric.value}</span>
            <span className="admin-summary__detail">{metric.detail}</span>
          </div>
        ))}
      </div>

      <div className="admin-data-panel">
        {toolbar}
        {children}
      </div>
    </section>
  )
}
