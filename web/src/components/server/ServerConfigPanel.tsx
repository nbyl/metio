import { Settings2, Globe, Cpu, HardDrive, Calendar } from 'lucide-react';
import type { ServerConfig } from '../../types/server';
import { Card, CardContent } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { Separator } from '../ui/Separator';
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '../ui/Collapsible';
import { cn } from '../../lib/utils';

export interface ServerConfigPanelProps {
  config: ServerConfig;
  infrahVersion: number;
  outdated: boolean;
  compact?: boolean;
  className?: string;
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleDateString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return iso;
  }
}

export function ServerConfigPanel({
  config,
  infrahVersion,
  outdated,
  compact,
  className,
}: ServerConfigPanelProps) {
  if (compact) {
    return (
      <Collapsible className={cn('space-y-3', className)}>
        <CollapsibleTrigger className="flex w-full items-center justify-between py-2 text-left">
          <span className="flex items-center gap-2 text-sm font-medium text-foreground">
            <Settings2 className="h-4 w-4 text-muted-foreground" />
            Configuration
          </span>
          {outdated && <Badge variant="secondary">Update Available</Badge>}
        </CollapsibleTrigger>
        <CollapsibleContent className="space-y-3 pt-3">
          <Separator />
          <dl className="grid grid-cols-1 gap-3 text-sm">
            <div>
              <dt className="flex items-center gap-2 font-medium text-muted-foreground">
                <Globe className="h-3.5 w-3.5" />
                Region
              </dt>
              <dd className="text-foreground">
                {config.region}/{config.zone}
              </dd>
            </div>
            <div>
              <dt className="flex items-center gap-2 font-medium text-muted-foreground">
                <Cpu className="h-3.5 w-3.5" />
                Machine Type
              </dt>
              <dd className="text-foreground">{config.machineType}</dd>
            </div>
            <div>
              <dt className="font-medium text-muted-foreground">
                Minecraft Version
              </dt>
              <dd className="text-foreground">{config.minecraftVersion}</dd>
            </div>
            <div>
              <dt className="flex items-center gap-2 font-medium text-muted-foreground">
                <HardDrive className="h-3.5 w-3.5" />
                Disk Size
              </dt>
              <dd className="text-foreground">{config.diskSizeGB} GB</dd>
            </div>
          </dl>
        </CollapsibleContent>
      </Collapsible>
    );
  }

  return (
    <Card className={cn(className)}>
      <CardContent>
        <Collapsible>
          <CollapsibleTrigger className="flex w-full items-center justify-between py-2 text-left">
            <span className="flex items-center gap-2 text-sm font-medium text-foreground">
              <Settings2 className="h-4 w-4" />
              Server Configuration
            </span>
            {outdated && <Badge variant="secondary">Update Available</Badge>}
          </CollapsibleTrigger>
          <CollapsibleContent className="space-y-3 pt-3">
            <Separator />
            <dl className="grid grid-cols-1 gap-3 text-sm md:grid-cols-2">
              <div>
                <dt className="flex items-center gap-2 font-medium text-muted-foreground">
                  <Globe className="h-3.5 w-3.5" />
                  Name
                </dt>
                <dd className="text-foreground">{config.name}</dd>
              </div>
              <div>
                <dt className="flex items-center gap-2 font-medium text-muted-foreground">
                  <Globe className="h-3.5 w-3.5" />
                  Location
                </dt>
                <dd className="text-foreground">
                  {config.region}/{config.zone}
                </dd>
              </div>
              <div>
                <dt className="flex items-center gap-2 font-medium text-muted-foreground">
                  <Cpu className="h-3.5 w-3.5" />
                  Machine Type
                </dt>
                <dd className="text-foreground">{config.machineType}</dd>
              </div>
              <div>
                <dt className="flex items-center gap-2 font-medium text-muted-foreground">
                  <HardDrive className="h-3.5 w-3.5" />
                  Disk Size
                </dt>
                <dd className="text-foreground">{config.diskSizeGB} GB</dd>
              </div>
              <div>
                <dt className="font-medium text-muted-foreground">
                  Minecraft Version
                </dt>
                <dd className="text-foreground">{config.minecraftVersion}</dd>
              </div>
              <div>
                <dt className="font-medium text-muted-foreground">
                  Infra Version
                </dt>
                <dd className="text-foreground">v{infrahVersion}</dd>
              </div>
            </dl>
            <Separator />
            <div className="space-y-1 text-xs text-muted-foreground">
              <div className="flex items-center gap-1">
                <Calendar className="h-3 w-3" />
                Created: {formatDate(config.createdAt)}
              </div>
              <div className="flex items-center gap-1">
                <Calendar className="h-3 w-3" />
                Updated: {formatDate(config.updatedAt)}
              </div>
            </div>
          </CollapsibleContent>
        </Collapsible>
      </CardContent>
    </Card>
  );
}
