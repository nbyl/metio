import { useState } from 'react';
import {
  AlertTriangle,
  ArrowLeft,
  Database,
  Globe,
  HardDrive,
  Loader2,
  Terminal,
  X,
} from 'lucide-react';
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
import { Input } from '../ui/Input';
import { Label } from '../ui/Label';
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
}: {
  serverName: string;
  onNext: () => void;
}) {
  return (
    <div className="space-y-5">
      <div className="flex items-center gap-3 rounded-lg border border-destructive/30 bg-destructive/10 px-4 py-3">
        <AlertTriangle className="h-5 w-5 shrink-0 text-destructive" />
        <p className="text-sm text-foreground">
          This action will permanently destroy{' '}
          <strong className="text-destructive">{serverName}</strong> and all
          associated resources. This cannot be undone!
        </p>
      </div>

      <div className="space-y-2">
        <p className="text-sm font-medium text-foreground">
          The following will be deleted:
        </p>
        <ul className="space-y-2">
          <li className="flex items-center gap-3 text-sm text-foreground">
            <Terminal className="h-4 w-4 text-muted-foreground" />
            The Minecraft server VM
          </li>
          <li className="flex items-center gap-3 text-sm text-foreground">
            <HardDrive className="h-4 w-4 text-muted-foreground" />
            All world data and configurations
          </li>
          <li className="flex items-center gap-3 text-sm text-foreground">
            <Database className="h-4 w-4 text-muted-foreground" />
            Backup bucket and backups
          </li>
          <li className="flex items-center gap-3 text-sm text-foreground">
            <Globe className="h-4 w-4 text-muted-foreground" />
            Static IP address
          </li>
        </ul>
      </div>

      <AlertDialogFooter className="mx-0 mb-0 rounded-none border-0 bg-transparent p-0">
        <AlertDialogCancel>Cancel</AlertDialogCancel>
        <Button type="button" variant="destructive" onClick={onNext}>
          Continue →
        </Button>
      </AlertDialogFooter>
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
        <Label htmlFor="destroy-confirm" className="text-destructive">
          Type the server name to confirm:
        </Label>
        <Input
          id="destroy-confirm"
          type="text"
          value={confirmText}
          onChange={(e) => setConfirmText(e.target.value)}
          placeholder={serverName}
          className="focus-visible:ring-destructive/40"
        />
      </div>

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
          disabled={!isConfirmed || isPending}
          onClick={onConfirm}
        >
          {isPending && <Loader2 className="animate-spin" />}
          Destroy Server
        </Button>
      </AlertDialogFooter>
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
  const totalSteps = 1;

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
          disabled={isPending}
        >
          <X />
        </AlertDialogCancel>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-destructive" />
            Destroy Server
          </AlertDialogTitle>
          <AlertDialogDescription className="sr-only">
            Permanently destroy {serverName} and its associated resources.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="mb-2 flex gap-1">
          {Array.from({ length: totalSteps + 1 }).map((_, index) => (
            <div
              key={index}
              className={cn(
                'h-1 flex-1 rounded-full transition-colors duration-300',
                index <= step ? 'bg-red-500' : 'bg-muted'
              )}
            />
          ))}
        </div>

        {step === 0 && (
          <WarningStep serverName={serverName} onNext={handleNext} />
        )}
        {step === totalSteps && (
          <ConfirmStep
            serverName={serverName}
            isPending={isPending}
            onConfirm={onConfirm}
            onBack={handleBack}
          />
        )}
      </AlertDialogContent>
    </AlertDialog>
  );
}
