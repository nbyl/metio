import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import {
  ArrowLeft,
  ArrowRight,
  Check,
  Cpu,
  FileText,
  Loader2,
  Package,
  Search,
  Server,
  Settings,
} from 'lucide-react';
import { useServerOptions } from '../../hooks/useServerOptions';
import { useModrinthSearch } from '../../hooks/useModrinthSearch';
import { useModrinthPackVersions } from '../../hooks/useModrinthPackVersions';
import { useCreateServer } from '../../hooks/useServerMutations';
import { Card, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
import { Badge } from '../ui/Badge';
import { Skeleton } from '../ui/Skeleton';
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '../ui/Form';
import { Input } from '../ui/Input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/Select';
import { Switch } from '../ui/Switch';
import { cn } from '../../lib/utils';
import type {
  MachineTypeOption,
  ModpackConfig,
  ModrinthPack,
} from '../../types/server';

export interface ServerSetupWizardProps {
  className?: string;
}

const serverNamePattern = /^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z][a-z0-9]$|^[a-z]$/;

/**
 * A modpack selection made in the wizard. Display fields (name, versionName)
 * are carried for the Review step; only platform/projectId/versionId are sent
 * to the API. An empty versionId installs the pack's latest version.
 */
type ModpackSelection = {
  platform: 'modrinth';
  projectId: string;
  name: string;
  iconUrl?: string;
  loader?: string;
  versionId?: string;
  versionName?: string;
};

const modpackSchema = z
  .object({
    platform: z.literal('modrinth'),
    projectId: z.string().min(1),
    name: z.string().min(1),
    iconUrl: z.string().optional(),
    loader: z.string().optional(),
    versionId: z.string().optional(),
    versionName: z.string().optional(),
  })
  .nullable();

const wizardSchema = z
  .object({
    name: z
      .string()
      .min(1, 'Server name is required')
      .min(3, 'Name must be between 3 and 24 characters')
      .max(24, 'Name must be between 3 and 24 characters')
      .regex(
        serverNamePattern,
        'Name must start with a letter and contain only lowercase letters, digits, and hyphens'
      ),
    region: z.string().min(1, 'Region is required'),
    zone: z.string().min(1, 'Zone is required'),
    machineType: z.string().min(1, 'Machine type is required'),
    minecraftVersion: z.string(),
    diskSizeGB: z.number().min(10).max(100),
    shutdownEnabled: z.boolean(),
    shutdownTime: z.string(),
    shutdownTimezone: z.string(),
    modpack: modpackSchema,
  })
  .superRefine((values, ctx) => {
    // A pack controls the Minecraft version (ADR-0006); only vanilla servers
    // must choose one.
    if (!values.modpack && !values.minecraftVersion) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['minecraftVersion'],
        message: 'Minecraft version is required',
      });
    }
  });

type WizardForm = z.infer<typeof wizardSchema>;

const STEPS = [
  { id: 0, label: 'Basic Info', icon: Server },
  { id: 1, label: 'Modpack', icon: Package },
  { id: 2, label: 'Server Specs', icon: Cpu },
  { id: 3, label: 'Settings', icon: Settings },
  { id: 4, label: 'Review', icon: FileText },
];

const SEARCH_MIN_LENGTH = 3;

const TIMEZONES = [
  'UTC',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Paris',
  'Europe/Moscow',
  'Asia/Tokyo',
  'Asia/Shanghai',
  'Asia/Dubai',
  'Australia/Sydney',
  'Pacific/Auckland',
];

const SHUTDOWN_TIMES = [
  '18:00',
  '19:00',
  '20:00',
  '21:00',
  '22:00',
  '23:00',
  '00:00',
];

const INITIAL_FORM: WizardForm = {
  name: '',
  region: '',
  zone: '',
  machineType: '',
  minecraftVersion: '',
  diskSizeGB: 20,
  shutdownEnabled: false,
  shutdownTime: '21:00',
  shutdownTimezone: 'Europe/Berlin',
  modpack: null,
};

const DEFAULT_FAMILIES = ['e2-', 'n2-'];
const STEP_FIELDS: (keyof WizardForm)[][] = [
  ['name', 'region', 'zone'],
  ['modpack'],
  ['machineType', 'minecraftVersion'],
  [],
  [],
];

/** Formats a raw download count into a compact label (e.g. 12.3M). */
function formatDownloads(count: number): string {
  if (count >= 1_000_000) {
    return `${(count / 1_000_000).toFixed(1).replace(/\.0$/, '')}M`;
  }
  if (count >= 1_000) {
    return `${(count / 1_000).toFixed(1).replace(/\.0$/, '')}K`;
  }
  return `${count}`;
}

/** Human-readable summary of a pack selection for the Review step. */
function modpackSummary(selection: ModpackSelection): string {
  const version = selection.versionName ?? selection.versionId;
  return version
    ? `${selection.name} · ${version}`
    : `${selection.name} · Latest`;
}

/** Pack icon with a fallback to a generic package glyph. */
function PackIcon({ iconUrl }: { iconUrl: string }) {
  const [failed, setFailed] = useState(false);

  if (!iconUrl || failed) {
    return (
      <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-muted">
        <Package className="h-5 w-5 text-muted-foreground" />
      </div>
    );
  }

  return (
    <img
      src={iconUrl}
      alt=""
      className="h-10 w-10 shrink-0 rounded-md object-cover"
      onError={() => setFailed(true)}
    />
  );
}

function BasicInfoStep({
  form,
  regions,
}: {
  form: ReturnType<typeof useForm<WizardForm>>;
  regions: { id: string; zones: string[] }[];
}) {
  const selectedRegion = regions.find(
    (region) => region.id === form.watch('region')
  );

  return (
    <div className="space-y-6">
      <FormField
        control={form.control}
        name="name"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Server Name</FormLabel>
            <FormControl>
              <Input {...field} placeholder="my-minecraft-server" />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <FormField
          control={form.control}
          name="region"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Region</FormLabel>
              <Select
                value={field.value}
                onValueChange={(value) => {
                  field.onChange(value);
                  form.setValue('zone', '');
                }}
              >
                <FormControl>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Select a region" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {regions.map((region) => (
                    <SelectItem key={region.id} value={region.id}>
                      {region.id}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="zone"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Zone</FormLabel>
              <Select
                value={field.value}
                onValueChange={field.onChange}
                disabled={!selectedRegion}
              >
                <FormControl>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Select a zone" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent>
                  {selectedRegion?.zones.map((zone) => (
                    <SelectItem key={zone} value={zone}>
                      {zone}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
    </div>
  );
}

function ModpackStep({
  form,
}: {
  form: ReturnType<typeof useForm<WizardForm>>;
}) {
  const [query, setQuery] = useState('');
  const selected = form.watch('modpack');
  const search = useModrinthSearch(query);
  const versions = useModrinthPackVersions(selected?.projectId ?? null);

  const trimmedQuery = query.trim();
  const packs = search.data ?? [];
  const versionOptions = versions.data ?? [];
  const isSearching =
    search.isLoading ||
    (trimmedQuery !== search.debouncedQuery &&
      trimmedQuery.length >= SEARCH_MIN_LENGTH);

  const selectPack = (pack: ModrinthPack) => {
    form.setValue(
      'modpack',
      {
        platform: 'modrinth',
        projectId: pack.id,
        name: pack.name,
        iconUrl: pack.iconUrl,
        loader: pack.loader || undefined,
      },
      { shouldDirty: true }
    );
    // A pack controls the Minecraft version (ADR-0006), so drop any earlier
    // vanilla pick to keep the two fields mutually exclusive.
    form.setValue('minecraftVersion', '');
  };

  const clearPack = () => {
    form.setValue('modpack', null, { shouldDirty: true });
  };

  const selectVersion = (value: string) => {
    if (!selected) return;

    if (value === 'latest') {
      form.setValue(
        'modpack',
        { ...selected, versionId: undefined, versionName: undefined },
        { shouldDirty: true }
      );
      return;
    }

    const version = versionOptions.find((option) => option.id === value);
    form.setValue(
      'modpack',
      {
        ...selected,
        versionId: value,
        versionName: version?.name ?? version?.versionNumber,
      },
      { shouldDirty: true }
    );
  };

  return (
    <div className="space-y-6">
      <div>
        <p className="font-medium text-foreground">Server Type</p>
        <p className="mt-1 text-sm text-muted-foreground">
          Install a published Modrinth modpack, or skip this step for a vanilla
          server.
        </p>
      </div>

      <div className="space-y-2">
        <label htmlFor="modpack-search" className="text-sm font-medium">
          Search Modpacks
        </label>
        <div className="relative">
          <Search className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            id="modpack-search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search Modrinth modpacks…"
            className="pl-9"
            autoComplete="off"
          />
        </div>
      </div>

      {selected ? (
        <div className="space-y-4 rounded-lg border border-green-600 bg-green-600/10 p-4">
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 items-center gap-3">
              <PackIcon iconUrl={selected.iconUrl ?? ''} />
              <div className="min-w-0">
                <div className="truncate font-medium text-foreground">
                  {selected.name}
                </div>
                <div className="text-sm text-muted-foreground">
                  {selected.loader ? `${selected.loader} · ` : ''}Modrinth
                </div>
              </div>
            </div>
            <Button type="button" variant="ghost" size="sm" onClick={clearPack}>
              Clear pack
            </Button>
          </div>

          <div className="space-y-2">
            <label htmlFor="modpack-version" className="text-sm font-medium">
              Pack Version
            </label>
            <Select
              value={selected.versionId ?? 'latest'}
              onValueChange={selectVersion}
              disabled={versions.isLoading}
            >
              <SelectTrigger id="modpack-version" className="w-full sm:w-72">
                <SelectValue
                  placeholder={
                    versions.isLoading ? 'Loading versions…' : 'Latest'
                  }
                />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="latest">Latest</SelectItem>
                {versionOptions.map((version) => (
                  <SelectItem key={version.id} value={version.id}>
                    {version.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {versions.isError ? (
              <p className="text-sm text-destructive">
                Failed to load pack versions.
              </p>
            ) : null}
          </div>
        </div>
      ) : null}

      {trimmedQuery.length < SEARCH_MIN_LENGTH ? (
        <p className="text-sm text-muted-foreground">
          Type at least {SEARCH_MIN_LENGTH} characters to search the Modrinth
          catalogue.
        </p>
      ) : isSearching ? (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {[0, 1, 2, 3].map((index) => (
            <Skeleton key={index} className="h-20 rounded-lg" />
          ))}
        </div>
      ) : search.isError ? (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          <p>Failed to search modpacks.</p>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="mt-3"
            onClick={() => search.refetch()}
          >
            Try again
          </Button>
        </div>
      ) : packs.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          No modpacks found. Try a different search.
        </p>
      ) : (
        <div
          className="grid grid-cols-1 gap-3 sm:grid-cols-2"
          role="group"
          aria-label="Modpack results"
        >
          {packs.map((pack) => {
            const isSelected = selected?.projectId === pack.id;
            return (
              <button
                key={pack.id}
                type="button"
                aria-pressed={isSelected}
                onClick={() => selectPack(pack)}
                className={cn(
                  'flex items-start gap-3 rounded-lg border p-3 text-left transition-all',
                  isSelected
                    ? 'border-green-600 bg-green-600/10 ring-1 ring-green-600'
                    : 'border-border bg-muted hover:border-ring'
                )}
              >
                <PackIcon iconUrl={pack.iconUrl} />
                <div className="min-w-0 flex-1">
                  <div className="truncate font-medium text-foreground">
                    {pack.name}
                  </div>
                  <div className="truncate text-sm text-muted-foreground">
                    by {pack.author}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2">
                    {pack.loader ? (
                      <Badge variant="secondary">{pack.loader}</Badge>
                    ) : null}
                    <span className="text-xs text-muted-foreground">
                      {formatDownloads(pack.downloads)} downloads
                    </span>
                  </div>
                </div>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

function SpecsStep({
  form,
  machineTypes,
  minecraftVersions,
}: {
  form: ReturnType<typeof useForm<WizardForm>>;
  machineTypes: MachineTypeOption[];
  minecraftVersions: string[];
}) {
  const [showAllMachineTypes, setShowAllMachineTypes] = useState(false);
  const selectedType = form.watch('machineType');
  const selectedPack = form.watch('modpack');
  const isDefaultFamily = (id: string) =>
    DEFAULT_FAMILIES.some((family) => id.startsWith(family));
  const visibleMachineTypes = showAllMachineTypes
    ? machineTypes
    : machineTypes.filter(
        (machine) => isDefaultFamily(machine.id) || machine.id === selectedType
      );
  const hiddenCount = machineTypes.length - visibleMachineTypes.length;

  return (
    <div className="space-y-8">
      <FormField
        control={form.control}
        name="machineType"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Machine Type</FormLabel>
            <FormControl>
              <div
                className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3"
                role="group"
                aria-label="Machine Type"
              >
                {visibleMachineTypes.map((machine) => {
                  const selected = field.value === machine.id;
                  return (
                    <button
                      key={machine.id}
                      type="button"
                      aria-pressed={selected}
                      onClick={() => field.onChange(machine.id)}
                      className={cn(
                        'rounded-lg border p-4 text-left transition-all',
                        selected
                          ? 'border-green-600 bg-green-600/10 ring-1 ring-green-600'
                          : 'border-border bg-muted hover:border-ring'
                      )}
                    >
                      <div className="font-medium text-foreground">
                        {machine.id}
                      </div>
                      <div className="mt-1 text-sm text-muted-foreground">
                        {machine.vcpus} vCPU · {machine.memoryGB} GB RAM
                      </div>
                    </button>
                  );
                })}
              </div>
            </FormControl>
            {hiddenCount > 0 || showAllMachineTypes ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => setShowAllMachineTypes((value) => !value)}
                className="mt-4"
              >
                {showAllMachineTypes
                  ? 'Show fewer machine types'
                  : `Show all machine types (${hiddenCount} more)`}
              </Button>
            ) : null}
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="minecraftVersion"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Minecraft Version</FormLabel>
            <Select
              value={field.value}
              onValueChange={field.onChange}
              disabled={Boolean(selectedPack)}
            >
              <FormControl>
                <SelectTrigger className="w-full">
                  <SelectValue
                    placeholder={
                      selectedPack
                        ? 'Controlled by modpack'
                        : 'Select a version'
                    }
                  />
                </SelectTrigger>
              </FormControl>
              <SelectContent>
                {minecraftVersions.map((version) => (
                  <SelectItem key={version} value={version}>
                    {version}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {selectedPack ? (
              <p className="text-sm text-muted-foreground">
                Controlled by the selected modpack ({selectedPack.name}).
              </p>
            ) : null}
            <FormMessage />
          </FormItem>
        )}
      />

      <FormField
        control={form.control}
        name="diskSizeGB"
        render={({ field }) => (
          <FormItem>
            <FormLabel>Disk Size: {field.value} GB</FormLabel>
            <FormControl>
              <Input
                type="range"
                min={10}
                max={100}
                value={field.value}
                onChange={(event) => field.onChange(Number(event.target.value))}
                className="accent-green-600"
              />
            </FormControl>
            <div className="flex justify-between text-xs text-muted-foreground">
              <span>10 GB</span>
              <span>100 GB</span>
            </div>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  );
}

function OptionsStep({
  form,
}: {
  form: ReturnType<typeof useForm<WizardForm>>;
}) {
  const shutdownEnabled = form.watch('shutdownEnabled');

  return (
    <div className="space-y-6">
      <FormField
        control={form.control}
        name="shutdownEnabled"
        render={({ field }) => (
          <FormItem className="flex items-center justify-between">
            <div>
              <FormLabel>Scheduled Shutdown</FormLabel>
              <p className="text-sm text-muted-foreground">
                Automatically stop the server at a set time
              </p>
            </div>
            <FormControl>
              <Switch
                checked={field.value}
                onCheckedChange={field.onChange}
                aria-label="Toggle scheduled shutdown"
              />
            </FormControl>
          </FormItem>
        )}
      />

      {shutdownEnabled && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <FormField
            control={form.control}
            name="shutdownTime"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Shutdown Time</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {SHUTDOWN_TIMES.map((time) => (
                      <SelectItem key={time} value={time}>
                        {time}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name="shutdownTimezone"
            render={({ field }) => (
              <FormItem>
                <FormLabel>Timezone</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {TIMEZONES.map((timezone) => (
                      <SelectItem key={timezone} value={timezone}>
                        {timezone}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />
        </div>
      )}
    </div>
  );
}

function ReviewStep({
  values,
  machineTypes,
}: {
  values: WizardForm;
  machineTypes: MachineTypeOption[];
}) {
  const selectedMachine = machineTypes.find(
    (machine) => machine.id === values.machineType
  );
  const rows = [
    ['Server Name', values.name],
    ['Region / Zone', `${values.region} / ${values.zone}`],
    ['Machine Type', values.machineType],
    ...(selectedMachine
      ? [
          [
            'vCPU / Memory',
            `${selectedMachine.vcpus} vCPU / ${selectedMachine.memoryGB} GB`,
          ],
        ]
      : []),
    [
      'Modpack',
      values.modpack ? modpackSummary(values.modpack) : 'Vanilla (no pack)',
    ],
    [
      'Minecraft Version',
      values.modpack ? 'Pack-controlled' : values.minecraftVersion,
    ],
    ['Disk Size', `${values.diskSizeGB} GB`],
    [
      'Scheduled Shutdown',
      values.shutdownEnabled
        ? `${values.shutdownTime} ${values.shutdownTimezone}`
        : 'Disabled',
    ],
  ];

  return (
    <dl className="divide-y divide-border rounded-lg border border-border bg-muted/50">
      {rows.map(([label, value]) => (
        <div
          key={label}
          className="flex items-center justify-between px-4 py-3"
        >
          <dt className="text-muted-foreground">{label}</dt>
          <dd className="font-medium text-foreground">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function ServerSetupWizard({ className }: ServerSetupWizardProps) {
  const navigate = useNavigate();
  const { data: options, isLoading: optionsLoading } = useServerOptions();
  const createServer = useCreateServer();
  const [step, setStep] = useState(0);
  const form = useForm<WizardForm>({
    resolver: zodResolver(wizardSchema),
    defaultValues: INITIAL_FORM,
    mode: 'onTouched',
  });

  const handleNext = async () => {
    const fields = STEP_FIELDS[step];
    const valid = fields.length === 0 || (await form.trigger(fields));
    if (valid && step < STEPS.length - 1) setStep((current) => current + 1);
  };

  const handleCreate = form.handleSubmit((values) => {
    const modpack: ModpackConfig | undefined = values.modpack
      ? {
          platform: values.modpack.platform,
          projectId: values.modpack.projectId,
          versionId: values.modpack.versionId,
        }
      : undefined;

    createServer.mutate(
      {
        name: values.name,
        region: values.region,
        zone: values.zone,
        machineType: values.machineType,
        minecraftVersion: values.modpack ? '' : values.minecraftVersion,
        diskSizeGB: values.diskSizeGB,
        ...(modpack ? { modpack } : {}),
        ...(values.shutdownEnabled
          ? {
              shutdownSchedule: {
                enabled: true,
                time: values.shutdownTime,
                timezone: values.shutdownTimezone,
              },
            }
          : {}),
      },
      {
        onSuccess: (data) => {
          navigate(`/servers/${data.id}/provisioning`, {
            state: { serverName: values.name },
          });
        },
      }
    );
  });

  if (optionsLoading) {
    return (
      <div className={cn('mx-auto max-w-4xl py-8', className)}>
        <div className="animate-pulse space-y-6">
          <div className="h-8 w-48 rounded bg-muted" />
          <div className="h-64 rounded-lg bg-muted" />
        </div>
      </div>
    );
  }

  if (!options) {
    return (
      <div className={cn('mx-auto max-w-4xl py-8 text-center', className)}>
        <p className="text-destructive">Failed to load server options</p>
        <Button
          variant="outline"
          onClick={() => navigate('/')}
          className="mt-4"
        >
          Back to Dashboard
        </Button>
      </div>
    );
  }

  return (
    <Form {...form}>
      <form
        className={cn('mx-auto max-w-4xl px-4 py-8', className)}
        onSubmit={handleCreate}
      >
        <h1 className="mb-8 text-2xl font-bold text-foreground">
          Create New Server
        </h1>

        <div className="mb-8 flex items-center">
          {STEPS.map((stepInfo, index) => (
            <div key={stepInfo.id} className="flex flex-1 items-center">
              <div className="flex items-center gap-2">
                <div
                  className={cn(
                    'flex h-8 w-8 items-center justify-center rounded-full text-sm font-medium transition-colors',
                    step === stepInfo.id
                      ? 'bg-primary text-primary-foreground'
                      : step > stepInfo.id
                        ? 'bg-primary/20 text-primary'
                        : 'bg-muted text-muted-foreground'
                  )}
                >
                  {step > stepInfo.id ? (
                    <Check className="h-4 w-4" />
                  ) : (
                    <stepInfo.icon className="h-4 w-4" />
                  )}
                </div>
                <span
                  className={cn(
                    'hidden text-sm sm:inline',
                    step === stepInfo.id
                      ? 'font-medium text-foreground'
                      : 'text-muted-foreground'
                  )}
                >
                  {stepInfo.label}
                </span>
              </div>
              {index < STEPS.length - 1 && (
                <div
                  className={cn(
                    'mx-3 h-px flex-1',
                    step > stepInfo.id ? 'bg-primary' : 'bg-border'
                  )}
                />
              )}
            </div>
          ))}
        </div>

        <Card>
          <CardContent>
            {step === 0 && (
              <BasicInfoStep form={form} regions={options.regions} />
            )}
            {step === 1 && <ModpackStep form={form} />}
            {step === 2 && (
              <SpecsStep
                form={form}
                machineTypes={options.machineTypes}
                minecraftVersions={options.minecraftVersions}
              />
            )}
            {step === 3 && <OptionsStep form={form} />}
            {step === 4 && (
              <ReviewStep
                values={form.getValues()}
                machineTypes={options.machineTypes}
              />
            )}
          </CardContent>
        </Card>

        <div className="mt-6 flex items-center justify-between">
          <Button type="button" variant="outline" onClick={() => navigate('/')}>
            Cancel
          </Button>
          <div className="flex items-center gap-3">
            {step > 0 && (
              <Button
                type="button"
                variant="outline"
                onClick={() => setStep((current) => current - 1)}
                disabled={createServer.isPending}
              >
                <ArrowLeft />
                Back
              </Button>
            )}
            {step < STEPS.length - 1 ? (
              <Button type="button" onClick={handleNext}>
                Next
                <ArrowRight />
              </Button>
            ) : (
              <Button type="submit" disabled={createServer.isPending}>
                {createServer.isPending && <Loader2 className="animate-spin" />}
                Create Server
              </Button>
            )}
          </div>
        </div>
      </form>
    </Form>
  );
}
