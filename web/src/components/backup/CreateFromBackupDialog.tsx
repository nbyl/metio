import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Loader2, X } from 'lucide-react';
import type { BackupRecord, CreateFromBackupRequest } from '../../types/server';
import { useCreateServerFromBackup } from '../../hooks/useBackups';
import { useServerOptions } from '../../hooks/useServerOptions';
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

export interface CreateFromBackupDialogProps {
  open: boolean;
  backup: BackupRecord | null;
  onClose: () => void;
}

export function CreateFromBackupDialog({
  open,
  backup,
  onClose,
}: CreateFromBackupDialogProps) {
  const navigate = useNavigate();
  const { data: options, isLoading: optionsLoading } = useServerOptions();
  const createMutation = useCreateServerFromBackup(backup?.id ?? '');

  const source = backup?.sourceConfig;

  const [name, setName] = useState('');
  const [region, setRegion] = useState(source?.region ?? '');
  const [zone, setZone] = useState(source?.zone ?? '');
  const [machineType, setMachineType] = useState(source?.machineType ?? '');
  const [minecraftVersion, setMinecraftVersion] = useState(
    source?.minecraftVersion ?? backup?.minecraftVersion ?? ''
  );
  const [diskSizeGB, setDiskSizeGB] = useState(source?.diskSizeGB ?? 10);

  const availableVersions = options?.minecraftVersions ?? [];
  const versionOptions =
    minecraftVersion && !availableVersions.includes(minecraftVersion)
      ? [minecraftVersion, ...availableVersions]
      : availableVersions;

  const availableMachineTypes = options?.machineTypes ?? [];
  const machineTypeOptions = availableMachineTypes.map((mt) => ({
    id: mt.id,
    label: `${mt.id} (${mt.vcpus} vCPU · ${mt.memoryGB} GB RAM)`,
  }));

  const regions = options?.regions ?? [];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!backup || !name.trim()) return;

    const request: CreateFromBackupRequest = { name: name.trim() };
    if (region) request.region = region;
    if (zone) request.zone = zone;
    if (machineType) request.machineType = machineType;
    if (minecraftVersion) request.minecraftVersion = minecraftVersion;
    if (diskSizeGB) request.diskSizeGB = diskSizeGB;

    createMutation.mutate(request, {
      onSuccess: (data: { id: string }) => {
        const serverId = data.id;
        onClose();
        navigate(`/servers/${serverId}/provisioning`, {
          state: { serverName: name.trim() },
        });
      },
    });
  };

  const handleClose = () => {
    setName('');
    onClose();
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) handleClose();
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogClose
          aria-label="Close"
          variant="ghost"
          size="icon-sm"
          className="absolute top-2 right-2"
          disabled={createMutation.isPending}
        >
          <X />
        </DialogClose>
        <DialogHeader>
          <DialogTitle>Create Server from Backup</DialogTitle>
          <DialogDescription className="sr-only">
            Create a new server pre-filled with configuration from this backup.
          </DialogDescription>
        </DialogHeader>

        {backup && (
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="server-name">Server Name</Label>
              <Input
                id="server-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. Survival World"
                autoFocus
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="region">Region</Label>
                <Select value={region} onValueChange={setRegion}>
                  <SelectTrigger id="region" disabled={optionsLoading}>
                    <SelectValue placeholder="Default" />
                  </SelectTrigger>
                  <SelectContent>
                    {regions.map((r) => (
                      <SelectItem key={r.id} value={r.id}>
                        {r.id}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-2">
                <Label htmlFor="zone">Zone</Label>
                <Select value={zone} onValueChange={setZone}>
                  <SelectTrigger id="zone" disabled={optionsLoading}>
                    <SelectValue placeholder="Default" />
                  </SelectTrigger>
                  <SelectContent>
                    {regions
                      .find((r) => r.id === region)
                      ?.zones.map((z) => (
                        <SelectItem key={z} value={z}>
                          {z}
                        </SelectItem>
                      ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <Label htmlFor="machine-type">Machine Type</Label>
              <Select value={machineType} onValueChange={setMachineType}>
                <SelectTrigger
                  id="machine-type"
                  className="w-full"
                  disabled={optionsLoading}
                >
                  <SelectValue placeholder="Default" />
                </SelectTrigger>
                <SelectContent>
                  {machineTypeOptions.map((mt) => (
                    <SelectItem key={mt.id} value={mt.id}>
                      {mt.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label htmlFor="minecraft-version">Minecraft Version</Label>
                <Select
                  value={minecraftVersion}
                  onValueChange={setMinecraftVersion}
                >
                  <SelectTrigger
                    id="minecraft-version"
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
            </div>

            <DialogFooter className="mx-0 mb-0 rounded-none border-0 bg-transparent p-0">
              <DialogClose asChild>
                <Button
                  type="button"
                  variant="outline"
                  disabled={createMutation.isPending}
                >
                  Cancel
                </Button>
              </DialogClose>
              <Button
                type="submit"
                disabled={!name.trim() || createMutation.isPending}
              >
                {createMutation.isPending && (
                  <Loader2 className="animate-spin" />
                )}
                Create Server
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
