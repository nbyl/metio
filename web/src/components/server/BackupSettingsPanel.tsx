import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Loader2 } from 'lucide-react'
import type { BackupSettings } from '../../types/server'
import {
  useBackupSettings,
  useUpdateBackupSettings,
} from '../../hooks/useBackupSettings'
import { Button } from '../ui/Button'
import { Switch } from '../ui/Switch'

export interface BackupSettingsPanelProps {
  serverId: string
  serverName: string
}

export type BackupSettingsFormState = {
  enabled: boolean
  backupSchedule: string
  keepLast: number
  keepHourly: number
  keepDaily: number
  keepWeekly: number
  keepMonthly: number
  keepYearly: number
}

type RetentionKey = Exclude<
  keyof BackupSettingsFormState,
  'enabled' | 'backupSchedule'
>

const RETENTION_FIELDS: Array<{
  key: RetentionKey
  label: string
  hint: string
}> = [
  { key: 'keepLast', label: 'Keep last', hint: 'Always kept' },
  { key: 'keepHourly', label: 'Keep hourly', hint: 'Last N hours' },
  { key: 'keepDaily', label: 'Keep daily', hint: 'Last N days' },
  { key: 'keepWeekly', label: 'Keep weekly', hint: 'Last N weeks' },
  { key: 'keepMonthly', label: 'Keep monthly', hint: 'Last N months' },
  { key: 'keepYearly', label: 'Keep yearly', hint: 'Last N years' },
]

function toBackupFormState(settings: BackupSettings): BackupSettingsFormState {
  return {
    enabled: settings.enabled,
    backupSchedule: settings.backupSchedule ?? '',
    keepLast: settings.keepLast ?? 0,
    keepHourly: settings.keepHourly ?? 0,
    keepDaily: settings.keepDaily ?? 0,
    keepWeekly: settings.keepWeekly ?? 0,
    keepMonthly: settings.keepMonthly ?? 0,
    keepYearly: settings.keepYearly ?? 0,
  }
}

interface BackupSettingsFormProps {
  serverId: string
  serverName: string
  initial: BackupSettings
}

function BackupSettingsForm({
  serverId,
  serverName,
  initial,
}: BackupSettingsFormProps) {
  const navigate = useNavigate()
  const [form, setForm] = useState<BackupSettingsFormState>(() =>
    toBackupFormState(initial)
  )
  const updateMutation = useUpdateBackupSettings(serverId)

  const handleSetNumber = (key: RetentionKey, raw: string) => {
    if (raw === '') {
      setForm((prev) => ({ ...prev, [key]: 0 }))
      return
    }
    const value = Number(raw)
    if (Number.isNaN(value) || value < 0) return
    setForm((prev) => ({ ...prev, [key]: value }))
  }

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault()
    if (updateMutation.isPending) return

    const settings: BackupSettings = {
      enabled: form.enabled,
      backupSchedule: form.backupSchedule.trim() || undefined,
      keepLast: form.keepLast,
      keepHourly: form.keepHourly,
      keepDaily: form.keepDaily,
      keepWeekly: form.keepWeekly,
      keepMonthly: form.keepMonthly,
      keepYearly: form.keepYearly,
    }

    updateMutation.mutate(settings, {
      onSuccess: () => {
        navigate(`/servers/${serverId}/provisioning`, {
          state: { serverName },
        })
      },
    })
  }

  return (
    <form onSubmit={handleSave}>
      <div className="flex items-center justify-between mb-3">
        <Switch
          checked={form.enabled}
          onChange={(enabled) => setForm((prev) => ({ ...prev, enabled }))}
          disabled={updateMutation.isPending}
          aria-label="Toggle backups"
        />
        <span className="text-xs text-slate-400">
          {form.enabled ? 'Scheduled backups enabled' : 'Backups disabled'}
        </span>
      </div>

      <div className="backup-field mb-3">
        <label htmlFor="backup-schedule" className="backup-field-label">
          Backup interval
        </label>
        <input
          id="backup-schedule"
          type="text"
          value={form.backupSchedule}
          onChange={(e) =>
            setForm((prev) => ({
              ...prev,
              backupSchedule: e.target.value,
            }))
          }
          placeholder="e.g. 1h, 6h, 1d (default 1h)"
          className="whitelist-input"
          disabled={updateMutation.isPending || !form.enabled}
        />
      </div>

      <div className="backup-grid">
        {RETENTION_FIELDS.map(({ key, label, hint }) => (
          <div key={key} className="backup-field">
            <label htmlFor={`backup-${key}`} className="backup-field-label">
              {label}
            </label>
            <input
              id={`backup-${key}`}
              type="number"
              min={0}
              value={form[key] === 0 ? '' : form[key]}
              onChange={(e) => handleSetNumber(key, e.target.value)}
              placeholder="default"
              className="backup-number-input"
              disabled={updateMutation.isPending || !form.enabled}
            />
            <span className="backup-field-hint">{hint}</span>
          </div>
        ))}
      </div>

      <p className="text-xs text-slate-500 mb-3">
        Zero values use the deployment defaults. Saving re-provisions the server
        to apply the backup service changes.
      </p>

      <Button
        type="submit"
        variant="primary"
        disabled={updateMutation.isPending}
        loading={updateMutation.isPending}
        className="btn-sm"
      >
        Save
      </Button>
    </form>
  )
}

export function BackupSettingsPanel({
  serverId,
  serverName,
}: BackupSettingsPanelProps) {
  const { data, isLoading } = useBackupSettings(serverId)

  return (
    <div>
      {isLoading || !data ? (
        <div className="flex items-center justify-center py-8">
          <Loader2 className="h-5 w-5 animate-spin text-slate-400" />
        </div>
      ) : (
        <BackupSettingsForm
          serverId={serverId}
          serverName={serverName}
          initial={data}
        />
      )}
    </div>
  )
}
