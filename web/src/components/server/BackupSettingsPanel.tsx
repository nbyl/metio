import { useNavigate } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import type { BackupSettings } from '../../types/server';
import {
  useBackupSettings,
  useUpdateBackupSettings,
} from '../../hooks/useBackupSettings';
import { Button } from '../ui/Button';
import { Switch } from '../ui/Switch';
import { Input } from '../ui/Input';
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '../ui/Form';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/Select';

export interface BackupSettingsPanelProps {
  serverId: string;
  serverName: string;
}

export interface BackupSettingsFormState {
  enabled: boolean;
  backupIntervalHours: number;
  keep: number;
  keepUnit: BackupUnit;
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

const backupSettingsSchema = z.object({
  enabled: z.boolean(),
  backupIntervalHours: z.number().int().min(0).optional(),
  keep: z.number().int().min(0).max(3650).optional(),
  keepUnit: z.enum(KEEP_UNITS).optional(),
});

type BackupSettingsFormValues = z.infer<typeof backupSettingsSchema>;

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
  const form = useForm<BackupSettingsFormValues>({
    resolver: zodResolver(backupSettingsSchema),
    defaultValues: toBackupFormState(initial),
    mode: 'onTouched',
  });
  const enabled = useWatch({ control: form.control, name: 'enabled' });
  const keepUnit = useWatch({ control: form.control, name: 'keepUnit' });
  const updateMutation = useUpdateBackupSettings(serverId);

  const handleSave = form.handleSubmit((values) => {
    if (updateMutation.isPending) return;

    const settings: BackupSettings = {
      enabled: values.enabled,
      backupIntervalHours: values.backupIntervalHours || undefined,
      keep: values.keep || undefined,
      keepUnit: values.keep && values.keepUnit ? values.keepUnit : undefined,
    };

    updateMutation.mutate(settings, {
      onSuccess: () => {
        navigate(`/servers/${serverId}/provisioning`, {
          state: { serverName },
        });
      },
    });
  });

  return (
    <Form {...form}>
      <form onSubmit={handleSave} className="space-y-4">
        <FormField
          control={form.control}
          name="enabled"
          render={({ field }) => (
            <FormItem className="flex items-center justify-between">
              <FormLabel className="sr-only">Toggle backups</FormLabel>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                  disabled={updateMutation.isPending}
                  aria-label="Toggle backups"
                />
              </FormControl>
              <span className="text-xs text-muted-foreground">
                {field.value ? 'Scheduled backups enabled' : 'Backups disabled'}
              </span>
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="backupIntervalHours"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Backup interval (hours)</FormLabel>
              <FormControl>
                <Input
                  type="number"
                  min={0}
                  value={field.value ?? ''}
                  onChange={(event) => {
                    const value = event.target.value;
                    if (value === '') {
                      form.setValue('backupIntervalHours', undefined, {
                        shouldDirty: true,
                        shouldValidate: true,
                      });
                      return;
                    }
                    const number = Number(value);
                    if (number >= 0) {
                      form.setValue('backupIntervalHours', number, {
                        shouldDirty: true,
                        shouldValidate: true,
                      });
                    }
                  }}
                  placeholder="default (1h)"
                  disabled={updateMutation.isPending || !enabled}
                />
              </FormControl>
              <FormDescription>
                Hours between backups, e.g. 1, 6, 24 (default 1h)
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="keep"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Retention policy</FormLabel>
              <FormControl>
                <Input
                  type="number"
                  min={0}
                  max={3650}
                  value={field.value ?? ''}
                  onChange={(event) => {
                    const value = event.target.value;
                    if (value === '') {
                      form.setValue('keep', undefined, {
                        shouldDirty: true,
                        shouldValidate: true,
                      });
                      return;
                    }
                    const number = Number(value);
                    if (number >= 0) {
                      form.setValue('keep', number, {
                        shouldDirty: true,
                        shouldValidate: true,
                      });
                    }
                  }}
                  placeholder="default"
                  disabled={updateMutation.isPending || !enabled}
                />
              </FormControl>
              <FormDescription>
                {UNIT_HINTS[keepUnit ?? 'daily']}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="keepUnit"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Retention unit</FormLabel>
              <Select
                value={field.value}
                onValueChange={field.onChange}
                disabled={updateMutation.isPending || !enabled}
              >
                <FormControl>
                  <SelectTrigger aria-label="Retention unit">
                    <SelectValue />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {KEEP_UNITS.map((unit) => (
                    <SelectItem key={unit} value={unit}>
                      {UNIT_LABELS[unit]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <p className="text-xs text-muted-foreground mb-3">
          Zero values use the deployment defaults. Saving re-provisions the
          server to apply the backup service changes.
        </p>

        <Button
          type="submit"
          variant="default"
          disabled={updateMutation.isPending}
          size="sm"
        >
          {updateMutation.isPending && <Loader2 className="animate-spin" />}
          Save
        </Button>
      </form>
    </Form>
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
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
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
