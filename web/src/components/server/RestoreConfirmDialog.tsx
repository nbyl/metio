import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  AlertTriangle,
  ArrowLeft,
  Database,
  FileCheck,
  Gamepad2,
  HardDrive,
  Loader2,
  X,
} from 'lucide-react';
import type { BackupRecord } from '../../types/server';
import { useRestoreBackup } from '../../hooks/useBackups';
import { Button } from '../ui/Button';
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '../ui/AlertDialog';
import { cn } from '../../lib/utils';

export interface RestoreConfirmDialogProps {
  open: boolean;
  backup: BackupRecord | null;
  serverId: string;
  serverName: string;
  currentMinecraftVersion?: string;
  onClose: () => void;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return remaining > 0 ? `${minutes}m ${remaining}s` : `${minutes}m`;
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

function InfoStep({
  backup,
  currentMinecraftVersion,
  onNext,
  onBack,
}: {
  backup: BackupRecord;
  currentMinecraftVersion?: string;
  onNext: () => void;
  onBack: () => void;
}) {
  const versionMismatch =
    currentMinecraftVersion &&
    backup.minecraftVersion !== currentMinecraftVersion;

  return (
    <div className="space-y-5">
      {versionMismatch && (
        <div className="flex items-center gap-3 rounded-lg border border-yellow-500/30 bg-yellow-500/10 px-4 py-3">
          <AlertTriangle className="h-5 w-5 shrink-0 text-yellow-400" />
          <div className="text-sm">
            <p className="font-medium text-yellow-400">
              Minecraft Version Mismatch
            </p>
            <p className="text-yellow-300/70">
              Backup was created with <strong>{backup.minecraftVersion}</strong>{' '}
              but the server runs <strong>{currentMinecraftVersion}</strong>.
              Downgrading a world can cause data loss in blocks and items added
              by newer versions.
            </p>
          </div>
        </div>
      )}

      <div className="space-y-3">
        <p className="text-sm font-medium text-foreground">Snapshot details:</p>
        <div className="grid grid-cols-2 gap-3 text-sm">
          <div className="flex items-center gap-2">
            <Database className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">Created:</span>
            <span>{formatDate(backup.createdAt)}</span>
          </div>
          <div className="flex items-center gap-2">
            <FileCheck className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">Files:</span>
            <span>{backup.fileCount.toLocaleString()}</span>
          </div>
          <div className="flex items-center gap-2">
            <HardDrive className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">Size:</span>
            <span>{formatBytes(backup.repositorySize)}</span>
          </div>
          <div className="flex items-center gap-2">
            <Gamepad2 className="h-4 w-4 text-muted-foreground" />
            <span className="text-muted-foreground">Version:</span>
            <span>{backup.minecraftVersion}</span>
          </div>
        </div>
        <p className="text-xs text-muted-foreground">
          Duration: {formatDuration(backup.durationSeconds)}
        </p>
      </div>

      <div className="flex items-start gap-3 rounded-lg border border-border/60 bg-muted/40 px-4 py-3">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
        <p className="text-sm text-muted-foreground">
          The server will be stopped, the current world moved to a recovery
          directory, and the selected snapshot restored before the server
          restarts. This operation cannot be undone.
        </p>
      </div>

      <AlertDialogFooter className="mx-0 mb-0 rounded-none border-0 bg-transparent p-0">
        <Button type="button" variant="outline" onClick={onBack}>
          <ArrowLeft />
          Back
        </Button>
        <Button type="button" variant="destructive" onClick={onNext}>
          Continue →
        </Button>
      </AlertDialogFooter>
    </div>
  );
}

function ConfirmStep({
  backup,
  isPending,
  onConfirm,
  onBack,
}: {
  backup: BackupRecord;
  isPending: boolean;
  onConfirm: () => void;
  onBack: () => void;
}) {
  return (
    <div className="space-y-5">
      <p className="text-sm text-foreground">
        Are you sure you want to restore snapshot{' '}
        <strong>{backup.snapshotId.slice(0, 8)}</strong> from{' '}
        <strong>{formatDate(backup.createdAt)}</strong>?
      </p>

      <AlertDialogFooter className="mx-0 mb-0 rounded-none border-0 bg-transparent p-0">
        <Button
          type="button"
          variant="outline"
          onClick={onBack}
          disabled={isPending}
        >
          <ArrowLeft />
          Back
        </Button>
        <Button
          type="button"
          variant="destructive"
          disabled={isPending}
          onClick={onConfirm}
        >
          {isPending && <Loader2 className="animate-spin" />}
          Restore Backup
        </Button>
      </AlertDialogFooter>
    </div>
  );
}

export function RestoreConfirmDialog({
  open,
  backup,
  serverId,
  serverName,
  currentMinecraftVersion,
  onClose,
}: RestoreConfirmDialogProps) {
  const navigate = useNavigate();
  const [step, setStep] = useState(0);
  const totalSteps = 1;
  const restoreMutation = useRestoreBackup(serverId);

  const handleNext = () => {
    if (step < totalSteps) setStep((current) => current + 1);
  };

  const handleBack = () => {
    if (step > 0) setStep((current) => current - 1);
  };

  const handleClose = () => {
    setStep(0);
    onClose();
  };

  const handleConfirm = () => {
    if (!backup) return;
    restoreMutation.mutate(backup.id, {
      onSuccess: () => {
        navigate(`/servers/${serverId}/provisioning`, {
          state: { serverName },
        });
      },
    });
  };

  return (
    <AlertDialog
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) handleClose();
      }}
    >
      <AlertDialogContent size="sm" className="max-w-md">
        <AlertDialogCancel
          aria-label="Close"
          variant="ghost"
          size="icon-sm"
          className="absolute top-2 right-2"
          disabled={restoreMutation.isPending}
        >
          <X />
        </AlertDialogCancel>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2">
            <Database className="h-5 w-5 text-muted-foreground" />
            Restore Backup
          </AlertDialogTitle>
          <AlertDialogDescription className="sr-only">
            Restore a backup snapshot to {serverName}.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="mb-2 flex gap-1">
          {Array.from({ length: totalSteps + 1 }).map((_, index) => (
            <div
              key={index}
              className={cn(
                'h-1 flex-1 rounded-full transition-colors duration-300',
                index <= step ? 'bg-blue-500' : 'bg-muted'
              )}
            />
          ))}
        </div>

        {step === 0 && backup && (
          <InfoStep
            backup={backup}
            currentMinecraftVersion={currentMinecraftVersion}
            onNext={handleNext}
            onBack={handleClose}
          />
        )}
        {step === totalSteps && backup && (
          <ConfirmStep
            backup={backup}
            isPending={restoreMutation.isPending}
            onConfirm={handleConfirm}
            onBack={handleBack}
          />
        )}
      </AlertDialogContent>
    </AlertDialog>
  );
}
