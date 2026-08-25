import { useState } from 'react';
import { Loader2 } from 'lucide-react';
import type { ServerConfig, UpdateServerRequest } from '../../types/server';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog';
import { Input } from '../ui/Input';
import { Label } from '../ui/Label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/Select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../ui/Tabs';
import { useServerOptions } from '../../hooks/useServerOptions';
import { BackupSettingsPanel } from './BackupSettingsPanel';
import { BackupListPanel } from './BackupListPanel';

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

type SettingsTab = 'settings' | 'backup' | 'backups';

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

  // Always offer the current version so opening the modal cannot silently
  // switch the server when the API no longer lists that version.
  const availableVersions = options?.minecraftVersions ?? [];
  const versionOptions = availableVersions.includes(config.minecraftVersion)
    ? availableVersions
    : [config.minecraftVersion, ...availableVersions];

  // Keep the current machine type selectable when the dynamic list changes.
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
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) onClose();
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Server Settings</DialogTitle>
          <DialogDescription className="sr-only">
            Update settings for {serverName}.
          </DialogDescription>
        </DialogHeader>

        {outdated && (
          <div className="rounded-lg border border-yellow-500/30 bg-yellow-500/10 p-3 -mt-2 space-y-1">
            <p className="text-sm font-medium text-yellow-400">
              Infrastructure Update Available
            </p>
            <p className="text-xs text-yellow-300/70">
              Server version: v{config.infraVersion ?? '?'} → Controller
              version: v{currentInfraVersion}
            </p>
            <p className="text-xs text-muted-foreground">
              The server will continue running during the update. No
              configuration changes are required — submitting will trigger the
              upgrade.
            </p>
          </div>
        )}

        <Tabs
          value={activeTab}
          onValueChange={(value) => setActiveTab(value as SettingsTab)}
        >
          <TabsList variant="line">
            <TabsTrigger value="settings">Settings</TabsTrigger>
            <TabsTrigger value="backup">Backup</TabsTrigger>
            <TabsTrigger value="backups">Backups</TabsTrigger>
          </TabsList>
          <TabsContent value="settings" className="py-2">
            <form onSubmit={handleSubmit} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="server-name">Server Name</Label>
                <Input
                  id="server-name"
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="machine-type">Machine Type</Label>
                <Select value={machineType} onValueChange={setMachineType}>
                  <SelectTrigger
                    id="machine-type"
                    className="w-full"
                    disabled={optionsLoading}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {machineTypeOptions.map((mt) => (
                      <SelectItem key={mt.id} value={mt.id}>
                        {mt.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">
                  Changing the machine type will stop and restart the server.
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="disk-size">Disk Size (GB)</Label>
                <Input
                  id="disk-size"
                  type="number"
                  value={diskSizeGB}
                  onChange={(e) => setDiskSizeGB(Number(e.target.value))}
                  min={20}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="minecraft-version">Minecraft Version</Label>
                <Select
                  value={minecraftVersion}
                  onValueChange={setMinecraftVersion}
                >
                  <SelectTrigger
                    id="minecraft-version"
                    className="w-full"
                    disabled={optionsLoading}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {versionOptions.map((version) => (
                      <SelectItem key={version} value={version}>
                        {version}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <DialogFooter className="mx-0 mb-0 rounded-none border-0 bg-transparent p-0">
                <DialogClose asChild>
                  <Button type="button" variant="outline" disabled={isPending}>
                    Cancel
                  </Button>
                </DialogClose>
                <Button
                  type="submit"
                  disabled={(!hasChanges && !outdated) || isPending}
                >
                  {isPending && <Loader2 className="animate-spin" />}
                  Update Server
                </Button>
              </DialogFooter>
            </form>
          </TabsContent>
          <TabsContent value="backup" className="py-2">
            <BackupSettingsPanel serverId={serverId} serverName={serverName} />
          </TabsContent>
          <TabsContent value="backups" className="py-2">
            <BackupListPanel
              serverId={serverId}
              serverName={serverName}
              minecraftVersion={config.minecraftVersion}
            />
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
