import { useState } from 'react';
import {
  X,
  AlertTriangle,
  HardDrive,
  Database,
  Globe,
  Terminal,
} from 'lucide-react';
import { Button } from '../ui/Button';
import { cn } from '../../lib/utils';

export interface DestroyModalProps {
  open: boolean;
  serverName: string;
  onClose: () => void;
  onConfirm: () => void;
  isPending: boolean;
}

function WarningStep({
  serverName,
  onNext,
  onCancel,
}: {
  serverName: string;
  onNext: () => void;
  onCancel: () => void;
}) {
  return (
    <div className="space-y-5">
      <div className="flex items-center gap-3 rounded-lg bg-red-900/20 border border-red-800/50 px-4 py-3">
        <AlertTriangle className="h-5 w-5 text-red-400 shrink-0" />
        <p className="text-sm text-red-300">
          This action will permanently destroy{' '}
          <strong className="text-white">{serverName}</strong> and all
          associated resources. This cannot be undone!
        </p>
      </div>

      <div className="space-y-2">
        <p className="text-sm font-medium text-slate-300">
          The following will be deleted:
        </p>
        <ul className="space-y-2">
          <li className="flex items-center gap-3 text-sm text-slate-400">
            <Terminal className="h-4 w-4 text-slate-500" />
            The Minecraft server VM
          </li>
          <li className="flex items-center gap-3 text-sm text-slate-400">
            <HardDrive className="h-4 w-4 text-slate-500" />
            All world data and configurations
          </li>
          <li className="flex items-center gap-3 text-sm text-slate-400">
            <Database className="h-4 w-4 text-slate-500" />
            Backup bucket and backups
          </li>
          <li className="flex items-center gap-3 text-sm text-slate-400">
            <Globe className="h-4 w-4 text-slate-500" />
            Static IP address
          </li>
        </ul>
      </div>

      <div className="flex justify-end gap-3 pt-2">
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="button" variant="danger" onClick={onNext}>
          Continue →
        </Button>
      </div>
    </div>
  );
}

function ConfirmStep({
  serverName,
  isPending,
  onConfirm,
  onBack,
}: {
  serverName: string;
  isPending: boolean;
  onConfirm: () => void;
  onBack: () => void;
}) {
  const [confirmText, setConfirmText] = useState('');
  const isConfirmed = confirmText === serverName;

  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <p className="text-sm text-red-400 font-medium">
          Type the server name to confirm:
        </p>
        <input
          type="text"
          value={confirmText}
          onChange={(e) => setConfirmText(e.target.value)}
          placeholder={serverName}
          className="w-full px-3 py-2 text-sm bg-slate-700 border border-slate-600 rounded-md text-white placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-red-500 focus:border-transparent"
        />
      </div>

      <div className="flex justify-end gap-3 pt-2">
        <Button
          type="button"
          variant="outline"
          onClick={onBack}
          disabled={isPending}
        >
          ← Back
        </Button>
        <Button
          type="button"
          variant="danger"
          disabled={!isConfirmed || isPending}
          loading={isPending}
          onClick={onConfirm}
        >
          Destroy Server
        </Button>
      </div>
    </div>
  );
}

export function DestroyModal({
  open,
  serverName,
  onClose,
  onConfirm,
  isPending,
}: DestroyModalProps) {
  const [step, setStep] = useState(0);

  if (!open) return null;

  const totalSteps = 1;

  const handleNext = () => {
    if (step < totalSteps) {
      setStep(step + 1);
    }
  };

  const handleBack = () => {
    if (step > 0) {
      setStep(step - 1);
    }
  };

  const handleClose = () => {
    setStep(0);
    onClose();
  };

  const handleConfirm = () => {
    onConfirm();
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
          <h2 className="text-lg font-semibold text-white flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-red-400" />
            Destroy Server
          </h2>
          <button
            type="button"
            onClick={handleClose}
            disabled={isPending}
            className="text-slate-400 hover:text-white transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="mb-2 px-6">
          <div className="flex gap-1">
            {Array.from({ length: totalSteps + 1 }).map((_, i) => (
              <div
                key={i}
                className={cn(
                  'h-1 flex-1 rounded-full transition-colors duration-300',
                  i <= step ? 'bg-red-500' : 'bg-slate-700'
                )}
              />
            ))}
          </div>
        </div>

        <div className="px-6 pb-6">
          {step === 0 && (
            <WarningStep
              serverName={serverName}
              onNext={handleNext}
              onCancel={handleClose}
            />
          )}
          {step === totalSteps && (
            <ConfirmStep
              serverName={serverName}
              isPending={isPending}
              onConfirm={handleConfirm}
              onBack={handleBack}
            />
          )}
        </div>
      </div>
    </div>
  );
}
