import { Navigate, Route, Routes } from 'react-router-dom'
import { AppLayout } from '../layouts/AppLayout'
import { DashboardPage } from '../pages/dashboard/DashboardPage'
import { LoginPage } from '../pages/login/LoginPage'
import { NotificationsPage } from '../pages/notifications/NotificationsPage'
import { BackupRecordsPage } from '../pages/backup-records/BackupRecordsPage'
import { BackupTasksPage } from '../pages/backup-tasks/BackupTasksPage'
import { RestoreRecordsPage } from '../pages/restore-records/RestoreRecordsPage'
import { VerificationRecordsPage } from '../pages/verification-records/VerificationRecordsPage'
import { ReplicationRecordsPage } from '../pages/replication-records/ReplicationRecordsPage'
import { TaskTemplatesPage } from '../pages/task-templates/TaskTemplatesPage'
import { UsersPage } from '../pages/admin/UsersPage'
import { ApiKeysPage } from '../pages/admin/ApiKeysPage'
import { GoogleDriveCallbackPage } from '../pages/storage-targets/GoogleDriveCallbackPage'
import { StorageTargetsPage } from '../pages/storage-targets/StorageTargetsPage'
import { SettingsPage } from '../pages/settings/SettingsPage'
import { AuditLogsPage } from '../pages/audit/AuditLogsPage'
import NodesPage from '../pages/nodes/NodesPage'
import { ProtectedRoute } from './ProtectedRoute'

export function RouterView() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/"
        element={
          <ProtectedRoute>
            <AppLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Navigate to="/dashboard" replace />} />
        <Route path="dashboard" element={<DashboardPage />} />
        <Route path="backup/tasks" element={<BackupTasksPage />} />
        <Route path="backup/records" element={<BackupRecordsPage />} />
        <Route path="restore/records" element={<RestoreRecordsPage />} />
        <Route path="verify/records" element={<VerificationRecordsPage />} />
        <Route path="replication/records" element={<ReplicationRecordsPage />} />
        <Route path="task-templates" element={<TaskTemplatesPage />} />
        <Route path="admin/users" element={<UsersPage />} />
        <Route path="admin/api-keys" element={<ApiKeysPage />} />
        <Route path="storage-targets" element={<StorageTargetsPage />} />
        <Route path="storage-targets/google-drive/callback" element={<GoogleDriveCallbackPage />} />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="settings/notifications" element={<NotificationsPage />} />
        <Route path="nodes" element={<NodesPage />} />
        <Route path="audit" element={<AuditLogsPage />} />
        <Route path="system-info" element={<Navigate to="/settings" replace />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
