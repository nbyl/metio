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
  Server,
  Settings,
} from 'lucide-react';
import { useServerOptions } from '../../hooks/useServerOptions';
import { useCreateServer } from '../../hooks/useServerMutations';
import { Card, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
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
import type { MachineTypeOption } from '../../types/server';

export interface ServerSetupWizardProps {
  className?: string;
}

const serverNamePattern = /^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z][a-z0-9]$|^[a-z]$/;

const wizardSchema = z.object({
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
  minecraftVersion: z.string().min(1, 'Minecraft version is required'),
  diskSizeGB: z.number().min(10).max(100),
  shutdownEnabled: z.boolean(),
  shutdownTime: z.string(),
  shutdownTimezone: z.string(),
});

type WizardForm = z.infer<typeof wizardSchema>;

const STEPS = [
  { id: 0, label: 'Basic Info', icon: Server },
  { id: 1, label: 'Server Specs', icon: Cpu },
  { id: 2, label: 'Settings', icon: Settings },
  { id: 3, label: 'Review', icon: FileText },
];

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
};

const DEFAULT_FAMILIES = ['e2-', 'n2-'];
const STEP_FIELDS: (keyof WizardForm)[][] = [
  ['name', 'region', 'zone'],
  ['machineType', 'minecraftVersion'],
  [],
  [],
];

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
            <Select value={field.value} onValueChange={field.onChange}>
              <FormControl>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="Select a version" />
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
    ['Minecraft Version', values.minecraftVersion],
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
    createServer.mutate(
      {
        name: values.name,
        region: values.region,
        zone: values.zone,
        machineType: values.machineType,
        minecraftVersion: values.minecraftVersion,
        diskSizeGB: values.diskSizeGB,
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
            {step === 1 && (
              <SpecsStep
                form={form}
                machineTypes={options.machineTypes}
                minecraftVersions={options.minecraftVersions}
              />
            )}
            {step === 2 && <OptionsStep form={form} />}
            {step === 3 && (
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
