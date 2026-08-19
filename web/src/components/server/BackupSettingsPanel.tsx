import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import type { BackupSettings } from '../../types/server';
import {
  useBackupSettings,
  useUpdateBackupSettings,
} from '../../hooks/useBackupSettings';
import { Button } from '../ui/Button';
import { Switch } from '../ui/Switch';

export interface BackupSettingsPanelProps {
  serverId: string;
  serverName: string;
}

export interface BackupSettingsFormState {
  enabled: boolean;
  backupIntervalHours: number;
  keep: number;
  keepUnit: string;
}

const KEEP_UNITS = ['hourly', 'daily', 'weekly', 'monthly', 'yearly'] as const;

export type BackupUnit = (typeof KEEP_UNITS)[number];

const UNIT_LABELS: Record<BackupUnit, string> = {
  hourly: 'Hourly',
  daily: 'Daily',
  weekly: 'Weekly',
  monthly: 'Monthly',
  yearly: 'Yearly',
};

const UNIT_HINTS: Record<BackupUnit, string> = {
  hourly: 'Keep the last N hourly snapshots',
  daily: 'Keep the last N daily snapshots',
  weekly: 'Keep the last N weekly snapshots',
  monthly: 'Keep the last N monthly snapshots',
  yearly: 'Keep the last N yearly snapshots',
};

function toBackupFormState(settings: BackupSettings): BackupSettingsFormState {
  return {
    enabled: settings.enabled,
    backupIntervalHours: settings.backupIntervalHours ?? 0,
    keep: settings.keep ?? 0,
    keepUnit: (settings.keepUnit as BackupUnit | undefined) ?? 'daily',
  };
}

interface BackupSettingsFormProps {
  serverId: string;
  serverName: string;
  initial: BackupSettings;
}

function BackupSettingsForm({
  serverId,
  serverName,
  initial,
}: BackupSettingsFormProps) {
  const navigate = useNavigate();
  const [form, setForm] = useState<BackupSettingsFormState>(() =>
    toBackupFormState(initial)
  );
  const updateMutation = useUpdateBackupSettings(serverId);

  const handleSave = (e: React.FormEvent) => {
    e.preventDefault();
    if (updateMutation.isPending) return;

    const settings: BackupSettings = {
      enabled: form.enabled,
      backupIntervalHours: form.backupIntervalHours || undefined,
      keep: form.keep || undefined,
      keepUnit: form.keep && form.keepUnit ? form.keepUnit : undefined,
    };

    updateMutation.mutate(settings, {
      onSuccess: () => {
        navigate(`/servers/${serverId}/provisioning`, {
          state: { serverName },
        });
      },
    });
  };

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
        <label htmlFor="backup-interval" className="backup-field-label">
          Backup interval (hours)
        </label>
        <input
          id="backup-interval"
          type="number"
          min={0}
          value={form.backupIntervalHours === 0 ? '' : form.backupIntervalHours}
          onChange={(e) => {
            if (e.target.value === '') {
              setForm((prev) => ({ ...prev, backupIntervalHours: 0 }));
              return;
            }
            const value = Number(e.target.value);
            if (Number.isNaN(value) || value < 0) return;
            setForm((prev) => ({ ...prev, backupIntervalHours: value }));
          }}
          placeholder="default (1h)"
          className="backup-number-input"
          disabled={updateMutation.isPending || !form.enabled}
        />
        <span className="backup-field-hint">
          Hours between backups, e.g. 1, 6, 24 (default 1h)
        </span>
      </div>

      <div className="backup-field mb-3">
        <label htmlFor="backup-keep" className="backup-field-label">
          Retention policy
        </label>
        <div className="flex gap-2">
          <input
            id="backup-keep"
            type="number"
            min={0}
            value={form.keep === 0 ? '' : form.keep}
            onChange={(e) => {
              if (e.target.value === '') {
                setForm((prev) => ({ ...prev, keep: 0 }));
                return;
              }
              const value = Number(e.target.value);
              if (Number.isNaN(value) || value < 0) return;
              setForm((prev) => ({ ...prev, keep: value }));
            }}
            placeholder="default"
            className="backup-number-input"
            disabled={updateMutation.isPending || !form.enabled}
          />
          <select
            value={form.keepUnit}
            onChange={(e) =>
              setForm((prev) => ({ ...prev, keepUnit: e.target.value }))
            }
            className="whitelist-input"
            disabled={updateMutation.isPending || !form.enabled}
            aria-label="Retention unit"
          >
            {KEEP_UNITS.map((unit) => (
              <option key={unit} value={unit}>
                {UNIT_LABELS[unit]}
              </option>
            ))}
          </select>
        </div>
        <span className="backup-field-hint">
          {UNIT_HINTS[form.keepUnit as BackupUnit]}
        </span>
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
  );
}

export function BackupSettingsPanel({
  serverId,
  serverName,
}: BackupSettingsPanelProps) {
  const { data, isLoading } = useBackupSettings(serverId);

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
  );
}
