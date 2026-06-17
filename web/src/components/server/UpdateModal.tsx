import { useState } from 'react';
import { X } from 'lucide-react';
import type { ServerConfig, UpdateServerRequest } from '../../types/server';
import { Button } from '../ui/Button';
import { cn } from '../../lib/utils';

export interface UpdateModalProps {
  open: boolean;
  config: ServerConfig;
  currentInfraVersion: number;
  outdated: boolean;
  onClose: () => void;
  onUpdate: (data: UpdateServerRequest) => void;
  isPending: boolean;
}

export function UpdateModal({
  open,
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
          'w-full max-w-md mx-4'
        )}
      >
        <div className="flex items-center justify-between px-6 pt-6 pb-4">
          <h2 className="text-lg font-semibold text-white">Update Server</h2>
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
                Server version: v{config.infraVersion ?? '?'} → Controller version: v{currentInfraVersion}
              </p>
              <p className="text-xs text-slate-400">
                The server will continue running during the update. No configuration changes are
                required — submitting will trigger the upgrade.
              </p>
            </div>
          </div>
        )}

        <form onSubmit={handleSubmit} className="px-6 pb-6 space-y-4">
          <div className="space-y-2">
            <label className="text-sm text-slate-300 block">Server Name</label>
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
            <input
              type="text"
              value={machineType}
              onChange={(e) => setMachineType(e.target.value)}
              placeholder="e.g. e2-standard-2"
              className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-green-500 focus:border-transparent"
            />
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
            <input
              type="text"
              value={minecraftVersion}
              onChange={(e) => setMinecraftVersion(e.target.value)}
              placeholder="e.g. 1.20.4"
              className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-green-500 focus:border-transparent"
            />
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
              variant="primary"
              disabled={(!hasChanges && !outdated) || isPending}
              loading={isPending}
            >
              Update Server
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
