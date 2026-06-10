import { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, ArrowRight, Check, Server, Cpu, Settings, FileText } from 'lucide-react';
import { useServerOptions } from '../../hooks/useServerOptions';
import { useCreateServer } from '../../hooks/useServerMutations';
import { Card, CardContent } from '../ui/Card';
import { Button } from '../ui/Button';
import { Switch } from '../ui/Switch';
import { cn } from '../../lib/utils';

export interface ServerSetupWizardProps {
  className?: string;
}

interface WizardForm {
  name: string;
  region: string;
  zone: string;
  machineType: string;
  minecraftVersion: string;
  diskSizeGB: number;
  shutdownEnabled: boolean;
  shutdownTime: string;
  shutdownTimezone: string;
}

interface FormErrors {
  name?: string;
  region?: string;
  zone?: string;
  machineType?: string;
  minecraftVersion?: string;
  diskSizeGB?: string;
}

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

function validateStep(step: number, form: WizardForm): FormErrors {
  const errors: FormErrors = {};

  if (step === 0) {
    if (!form.name) {
      errors.name = 'Server name is required';
    } else if (form.name.length < 3 || form.name.length > 24) {
      errors.name = 'Name must be between 3 and 24 characters';
    } else if (!/^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z][a-z0-9]$|^[a-z]$/.test(form.name)) {
      errors.name = 'Name must start with a letter and contain only lowercase letters, digits, and hyphens';
    }

    if (!form.region) {
      errors.region = 'Region is required';
    }

    if (!form.zone) {
      errors.zone = 'Zone is required';
    }
  }

  if (step === 1) {
    if (!form.machineType) {
      errors.machineType = 'Machine type is required';
    }
    if (!form.minecraftVersion) {
      errors.minecraftVersion = 'Minecraft version is required';
    }
  }

  return errors;
}

function hasErrors(errors: FormErrors): boolean {
  return Object.keys(errors).length > 0;
}

function formatCost(cost: number): string {
  return `$${cost.toFixed(2)}`;
}

interface BasicInfoStepProps {
  form: WizardForm;
  errors: FormErrors;
  regions: { id: string; zones: string[] }[];
  onChange: (updates: Partial<WizardForm>) => void;
}

function BasicInfoStep({ form, errors, regions, onChange }: BasicInfoStepProps) {
  const selectedRegion = regions.find((r) => r.id === form.region);

  return (
    <div className="space-y-6">
      <div>
        <label className="block text-sm font-medium text-slate-300 mb-2">
          Server Name
        </label>
        <input
          type="text"
          value={form.name}
          onChange={(e) => onChange({ name: e.target.value })}
          placeholder="my-minecraft-server"
          className={cn(
            'w-full px-3 py-2 rounded-lg border bg-slate-700 text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500',
            errors.name ? 'border-red-500' : 'border-slate-600'
          )}
        />
        {errors.name && (
          <p className="mt-1 text-sm text-red-400">{errors.name}</p>
        )}
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-slate-300 mb-2">
            Region
          </label>
          <select
            value={form.region}
            onChange={(e) => onChange({ region: e.target.value, zone: '' })}
            className={cn(
              'w-full px-3 py-2 rounded-lg border bg-slate-700 text-white focus:outline-none focus:ring-2 focus:ring-blue-500',
              errors.region ? 'border-red-500' : 'border-slate-600'
            )}
          >
            <option value="">Select a region</option>
            {regions.map((r) => (
              <option key={r.id} value={r.id}>
                {r.id}
              </option>
            ))}
          </select>
          {errors.region && (
            <p className="mt-1 text-sm text-red-400">{errors.region}</p>
          )}
        </div>

        <div>
          <label className="block text-sm font-medium text-slate-300 mb-2">
            Zone
          </label>
          <select
            value={form.zone}
            onChange={(e) => onChange({ zone: e.target.value })}
            disabled={!form.region}
            className={cn(
              'w-full px-3 py-2 rounded-lg border bg-slate-700 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50',
              errors.zone ? 'border-red-500' : 'border-slate-600'
            )}
          >
            <option value="">Select a zone</option>
            {selectedRegion?.zones.map((z) => (
              <option key={z} value={z}>
                {z}
              </option>
            ))}
          </select>
          {errors.zone && (
            <p className="mt-1 text-sm text-red-400">{errors.zone}</p>
          )}
        </div>
      </div>
    </div>
  );
}

interface SpecsStepProps {
  form: WizardForm;
  errors: FormErrors;
  machineTypes: { id: string; vcpus: number; memoryGB: number; monthlyCost: number }[];
  minecraftVersions: string[];
  onChange: (updates: Partial<WizardForm>) => void;
}

function SpecsStep({ form, errors, machineTypes, minecraftVersions, onChange }: SpecsStepProps) {
  return (
    <div className="space-y-8">
      <div>
        <label className="block text-sm font-medium text-slate-300 mb-3">
          Machine Type
        </label>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {machineTypes.map((mt) => {
            const selected = form.machineType === mt.id;
            return (
              <button
                key={mt.id}
                type="button"
                onClick={() => onChange({ machineType: mt.id })}
                className={cn(
                  'text-left p-4 rounded-lg border transition-all',
                  selected
                    ? 'border-blue-500 bg-blue-500/10 ring-1 ring-blue-500'
                    : 'border-slate-600 bg-slate-700 hover:border-slate-500'
                )}
              >
                <div className="font-medium text-white">{mt.id}</div>
                <div className="mt-1 text-sm text-slate-400">
                  {mt.vcpus} vCPU · {mt.memoryGB} GB RAM
                </div>
                <div className="mt-1 text-sm text-slate-400">
                  {formatCost(mt.monthlyCost)}/mo
                </div>
              </button>
            );
          })}
        </div>
        {errors.machineType && (
          <p className="mt-2 text-sm text-red-400">{errors.machineType}</p>
        )}
      </div>

      <div>
        <label className="block text-sm font-medium text-slate-300 mb-2">
          Minecraft Version
        </label>
        <select
          value={form.minecraftVersion}
          onChange={(e) => onChange({ minecraftVersion: e.target.value })}
          className={cn(
            'w-full px-3 py-2 rounded-lg border bg-slate-700 text-white focus:outline-none focus:ring-2 focus:ring-blue-500',
            errors.minecraftVersion ? 'border-red-500' : 'border-slate-600'
          )}
        >
          <option value="">Select a version</option>
          {minecraftVersions.map((v) => (
            <option key={v} value={v}>
              {v}
            </option>
          ))}
        </select>
        {errors.minecraftVersion && (
          <p className="mt-1 text-sm text-red-400">{errors.minecraftVersion}</p>
        )}
      </div>

      <div>
        <label className="block text-sm font-medium text-slate-300 mb-2">
          Disk Size: {form.diskSizeGB} GB
        </label>
        <input
          type="range"
          min={10}
          max={100}
          value={form.diskSizeGB}
          onChange={(e) => onChange({ diskSizeGB: Number(e.target.value) })}
          className="w-full accent-blue-500"
        />
        <div className="flex justify-between text-xs text-slate-500 mt-1">
          <span>10 GB</span>
          <span>100 GB</span>
        </div>
      </div>
    </div>
  );
}

interface OptionsStepProps {
  form: WizardForm;
  onChange: (updates: Partial<WizardForm>) => void;
}

function OptionsStep({ form, onChange }: OptionsStepProps) {
  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <div className="text-sm font-medium text-slate-300">
            Scheduled Shutdown
          </div>
          <div className="text-sm text-slate-500">
            Automatically stop the server at a set time
          </div>
        </div>
        <Switch
          checked={form.shutdownEnabled}
          onChange={(enabled) => onChange({ shutdownEnabled: enabled })}
          aria-label="Toggle scheduled shutdown"
        />
      </div>

      {form.shutdownEnabled && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">
              Shutdown Time
            </label>
            <select
              value={form.shutdownTime}
              onChange={(e) => onChange({ shutdownTime: e.target.value })}
              className="w-full px-3 py-2 rounded-lg border border-slate-600 bg-slate-700 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              {SHUTDOWN_TIMES.map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-slate-300 mb-2">
              Timezone
            </label>
            <select
              value={form.shutdownTimezone}
              onChange={(e) => onChange({ shutdownTimezone: e.target.value })}
              className="w-full px-3 py-2 rounded-lg border border-slate-600 bg-slate-700 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              {TIMEZONES.map((tz) => (
                <option key={tz} value={tz}>
                  {tz}
                </option>
              ))}
            </select>
          </div>
        </div>
      )}
    </div>
  );
}

interface ReviewStepProps {
  form: WizardForm;
  machineTypes: { id: string; vcpus: number; memoryGB: number; monthlyCost: number }[];
}

function ReviewStep({ form, machineTypes }: ReviewStepProps) {
  const selectedMachine = machineTypes.find((mt) => mt.id === form.machineType);

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-slate-600 bg-slate-700/50 divide-y divide-slate-600">
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-slate-400">Server Name</span>
          <span className="text-white font-medium">{form.name}</span>
        </div>
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-slate-400">Region / Zone</span>
          <span className="text-white font-medium">
            {form.region} / {form.zone}
          </span>
        </div>
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-slate-400">Machine Type</span>
          <span className="text-white font-medium">{form.machineType}</span>
        </div>
        {selectedMachine && (
          <div className="flex items-center justify-between px-4 py-3">
            <span className="text-slate-400">vCPU / Memory</span>
            <span className="text-white font-medium">
              {selectedMachine.vcpus} vCPU / {selectedMachine.memoryGB} GB
            </span>
          </div>
        )}
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-slate-400">Minecraft Version</span>
          <span className="text-white font-medium">{form.minecraftVersion}</span>
        </div>
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-slate-400">Disk Size</span>
          <span className="text-white font-medium">{form.diskSizeGB} GB</span>
        </div>
        <div className="flex items-center justify-between px-4 py-3">
          <span className="text-slate-400">Scheduled Shutdown</span>
          <span className="text-white font-medium">
            {form.shutdownEnabled
              ? `${form.shutdownTime} ${form.shutdownTimezone}`
              : 'Disabled'}
          </span>
        </div>
      </div>

      {selectedMachine && (
        <Card>
          <CardContent>
            <div className="text-center py-4">
              <div className="text-sm text-slate-400 mb-1">
                Estimated Monthly Cost
              </div>
              <div className="text-3xl font-bold text-white">
                {formatCost(selectedMachine.monthlyCost)}
              </div>
              <div className="text-sm text-slate-500 mt-1">
                Server must be stopped when not in use to reduce costs
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

export function ServerSetupWizard({ className }: ServerSetupWizardProps) {
  const navigate = useNavigate();
  const { data: options, isLoading: optionsLoading } = useServerOptions();
  const createServer = useCreateServer();
  const [step, setStep] = useState(0);
  const [form, setForm] = useState<WizardForm>(INITIAL_FORM);
  const [touched, setTouched] = useState(false);

  const errors = useMemo(() => validateStep(step, form), [step, form]);

  const handleChange = (updates: Partial<WizardForm>) => {
    setForm((prev) => ({ ...prev, ...updates }));
  };

  const handleNext = () => {
    setTouched(true);
    const errs = validateStep(step, form);
    if (hasErrors(errs)) return;

    if (step === 0 && !form.region) return;
    if (step < STEPS.length - 1) {
      setStep((s) => s + 1);
      setTouched(false);
    }
  };

  const handleBack = () => {
    if (step > 0) {
      setStep((s) => s - 1);
      setTouched(false);
    }
  };

  const handleCancel = () => {
    navigate('/');
  };

  const handleCreate = async () => {
    const allErrors = { ...validateStep(0, form), ...validateStep(1, form) };
    if (hasErrors(allErrors)) {
      setStep(0);
      setTouched(true);
      return;
    }

    const payload = {
      name: form.name,
      region: form.region,
      zone: form.zone,
      machineType: form.machineType,
      minecraftVersion: form.minecraftVersion,
      diskSizeGB: form.diskSizeGB,
      ...(form.shutdownEnabled
        ? {
            shutdownSchedule: {
              enabled: true,
              time: form.shutdownTime,
              timezone: form.shutdownTimezone,
            },
          }
        : {}),
    };

    createServer.mutate(payload, {
      onSuccess: (data) => {
        navigate(`/servers/${data.id}/provisioning`, {
          state: { serverName: form.name },
        });
      },
    });
  };

  const isSubmitting = createServer.isPending;

  if (optionsLoading) {
    return (
      <div className={cn('max-w-4xl mx-auto py-8', className)}>
        <div className="animate-pulse space-y-6">
          <div className="h-8 w-48 bg-slate-700 rounded" />
          <div className="h-64 bg-slate-700 rounded-lg" />
        </div>
      </div>
    );
  }

  if (!options) {
    return (
      <div className={cn('max-w-4xl mx-auto py-8 text-center', className)}>
        <p className="text-red-400">Failed to load server options</p>
        <Button variant="outline" onClick={() => navigate('/')} className="mt-4">
          Back to Dashboard
        </Button>
      </div>
    );
  }

  const selectedMachine = options.machineTypes.find((mt) => mt.id === form.machineType);

  const estimatedCost = selectedMachine?.monthlyCost;

  return (
    <div className={cn('max-w-4xl mx-auto py-8 px-4', className)}>
      <h1 className="text-2xl font-bold text-white mb-8">Create New Server</h1>

      <div className="flex items-center mb-8">
        {STEPS.map((s, idx) => (
          <div key={s.id} className="flex items-center flex-1">
            <div className="flex items-center gap-2">
              <div
                className={cn(
                  'w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium transition-colors',
                  step === s.id
                    ? 'bg-blue-600 text-white'
                    : step > s.id
                      ? 'bg-green-600 text-white'
                      : 'bg-slate-700 text-slate-400'
                )}
              >
                {step > s.id ? (
                  <Check className="h-4 w-4" />
                ) : (
                  <s.icon className="h-4 w-4" />
                )}
              </div>
              <span
                className={cn(
                  'text-sm hidden sm:inline',
                  step === s.id ? 'text-white font-medium' : 'text-slate-400'
                )}
              >
                {s.label}
              </span>
            </div>
            {idx < STEPS.length - 1 && (
              <div
                className={cn(
                  'flex-1 h-px mx-3',
                  step > s.id ? 'bg-green-600' : 'bg-slate-600'
                )}
              />
            )}
          </div>
        ))}
      </div>

      <Card>
        <CardContent>
          {step === 0 && (
            <BasicInfoStep
              form={form}
              errors={touched ? errors : {}}
              regions={options.regions}
              onChange={handleChange}
            />
          )}
          {step === 1 && (
            <SpecsStep
              form={form}
              errors={touched ? errors : {}}
              machineTypes={options.machineTypes}
              minecraftVersions={options.minecraftVersions}
              onChange={handleChange}
            />
          )}
          {step === 2 && (
            <OptionsStep
              form={form}
              onChange={handleChange}
            />
          )}
          {step === 3 && (
            <ReviewStep
              form={form}
              machineTypes={options.machineTypes}
            />
          )}
        </CardContent>
      </Card>

      <div className="flex items-center justify-between mt-6">
        <Button variant="outline" onClick={handleCancel}>
          Cancel
        </Button>

        <div className="flex items-center gap-3">
          {step > 0 && (
            <Button variant="outline" onClick={handleBack} disabled={isSubmitting}>
              <ArrowLeft className="h-4 w-4" />
              Back
            </Button>
          )}

          {step < STEPS.length - 1 ? (
            <Button variant="primary" onClick={handleNext}>
              Next
              <ArrowRight className="h-4 w-4" />
            </Button>
          ) : (
            <Button
              variant="primary"
              onClick={handleCreate}
              loading={isSubmitting}
              disabled={isSubmitting}
            >
              Create Server{estimatedCost != null ? ` · ${formatCost(estimatedCost)}/mo` : ''}
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
