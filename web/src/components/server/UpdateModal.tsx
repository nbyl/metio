import { useState } from 'react';
import { Loader2, X } from 'lucide-react';
import type { ServerConfig, UpdateServerRequest } from '../../types/server';
import { Button } from '../ui/Button';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '../ui/Tabs';
import { useServerOptions } from '../../hooks/useServerOptions';
import { cn } from '../../lib/utils';
import { BackupSettingsPanel } from './BackupSettingsPanel';

export interface UpdateModalProps {
  open: boolean;
  serverId: string;
  serverName: string;
  config: ServerConfig;
  currentInfraVersion: number;
  outdated: boolean;
  onClose: () => void;
  onUpdate: (data: UpdateServerRequest) => void;
  isPending: boolean;
}

type SettingsTab = 'settings' | 'backup';

export function UpdateModal({
  open,
  serverId,
  serverName,
  config,
  currentInfraVersion,
  outdated,
  onClose,
  onUpdate,
  isPending,
}: UpdateModalProps) {
  const [name, setName] = useState(config.name);
  const [machineType, setMachineType] = useState(config.machineType);
  const [diskSizeGB, setDiskSizeGB] = useState(config.diskSizeGB);
  const [minecraftVersion, setMinecraftVersion] = useState(
    config.minecraftVersion
  );
  const [activeTab, setActiveTab] = useState<SettingsTab>('settings');
  const { data: options, isLoading: optionsLoading } = useServerOptions();

  // Always offer the server's current version, even if Mojang no longer lists
  // it, so opening the modal cannot silently switch the server to another
  // version.
  const availableVersions = options?.minecraftVersions ?? [];
  const versionOptions = availableVersions.includes(config.minecraftVersion)
    ? availableVersions
    : [config.minecraftVersion, ...availableVersions];

  // Same for machine types: keep the current type selectable even if the
  // dynamic list no longer offers it.
  const availableMachineTypes = options?.machineTypes ?? [];
  const machineTypeOptions = (() => {
    const withSpecs = availableMachineTypes.map((mt) => ({
      id: mt.id,
      label: `${mt.id} (${mt.vcpus} vCPU · ${mt.memoryGB} GB RAM)`,
    }));
    if (withSpecs.some((mt) => mt.id === config.machineType)) return withSpecs;
    return [
      { id: config.machineType, label: config.machineType },
      ...withSpecs,
    ];
  })();

  if (!open) return null;

  const hasChanges =
    name !== config.name ||
    machineType !== config.machineType ||
    diskSizeGB !== config.diskSizeGB ||
    minecraftVersion !== config.minecraftVersion;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const data: UpdateServerRequest = {};
    if (name !== config.name) data.name = name;
    if (machineType !== config.machineType) data.machineType = machineType;
    if (diskSizeGB !== config.diskSizeGB) data.diskSizeGB = diskSizeGB;
    if (minecraftVersion !== config.minecraftVersion)
      data.minecraftVersion = minecraftVersion;

    onUpdate(data);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div
        className={cn(
          'bg-slate-800 rounded-xl border border-slate-700 shadow-xl',
          'w-full max-w-lg mx-4'
        )}
      >
        <div className="flex items-center justify-between px-6 pt-6 pb-4">
          <h2 className="text-lg font-semibold text-white">Server Settings</h2>
          <button
            type="button"
            onClick={onClose}
            className="text-slate-400 hover:text-white transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {outdated && (
          <div className="px-6 pb-4">
            <div className="bg-yellow-500/10 border border-yellow-500/30 rounded-lg p-3 space-y-1">
              <p className="text-sm text-yellow-400 font-medium">
                Infrastructure Update Available
              </p>
              <p className="text-xs text-yellow-300/70">
                Server version: v{config.infraVersion ?? '?'} → Controller
                version: v{currentInfraVersion}
              </p>
              <p className="text-xs text-slate-400">
                The server will continue running during the update. No
                configuration changes are required — submitting will trigger the
                upgrade.
              </p>
            </div>
          </div>
        )}

        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as SettingsTab)}
          className="px-6"
        >
          <TabsList variant="line">
            <TabsTrigger value="settings">Settings</TabsTrigger>
            <TabsTrigger value="backup">Backup</TabsTrigger>
          </TabsList>
          <TabsContent value="settings" className="py-4">
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <label className="text-sm text-slate-300 block">
                  Server Name
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-green-500 focus:border-transparent"
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm text-slate-300 block">
                  Machine Type
                </label>
                <select
                  value={machineType}
                  onChange={(e) => setMachineType(e.target.value)}
                  disabled={optionsLoading}
                  className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-green-500 focus:border-transparent disabled:opacity-50"
                >
                  {machineTypeOptions.map((mt) => (
                    <option key={mt.id} value={mt.id}>
                      {mt.label}
                    </option>
                  ))}
                </select>
                <p className="text-xs text-slate-500">
                  Changing the machine type will stop and restart the server.
                </p>
              </div>

              <div className="space-y-2">
                <label className="text-sm text-slate-300 block">
                  Disk Size (GB)
                </label>
                <input
                  type="number"
                  value={diskSizeGB}
                  onChange={(e) => setDiskSizeGB(Number(e.target.value))}
                  min={20}
                  className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-green-500 focus:border-transparent"
                />
              </div>

              <div className="space-y-2">
                <label className="text-sm text-slate-300 block">
                  Minecraft Version
                </label>
                <select
                  value={minecraftVersion}
                  onChange={(e) => setMinecraftVersion(e.target.value)}
                  disabled={optionsLoading}
                  className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white focus:outline-none focus:ring-2 focus:ring-green-500 focus:border-transparent disabled:opacity-50"
                >
                  {versionOptions.map((v) => (
                    <option key={v} value={v}>
                      {v}
                    </option>
                  ))}
                </select>
              </div>

              <div className="flex justify-end gap-3 pt-2">
                <Button
                  type="button"
                  variant="outline"
                  onClick={onClose}
                  disabled={isPending}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  variant="default"
                  disabled={(!hasChanges && !outdated) || isPending}
                >
                  {isPending && <Loader2 className="animate-spin" />}
                  Update Server
                </Button>
              </div>
            </form>
          </TabsContent>
          <TabsContent value="backup" className="py-4">
            <BackupSettingsPanel serverId={serverId} serverName={serverName} />
          </TabsContent>
        </Tabs>
      </div>
    </div>
  );
}
