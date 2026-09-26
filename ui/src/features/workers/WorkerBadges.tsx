// What an operator must know about a worker's host at a glance: how safe its engine is,
// and whether its image is behind core.
import type { Worker } from '@/types';
import { Badge } from '@/components/ui/Badge';

export function EngineBadge({ worker }: { worker: Worker }) {
  if (!worker.engine) return null;
  return worker.engine.rootless ? (
    <Badge tone='success' size='sm' title={`Docker ${worker.engine.version}, rootless`}>
      rootless
    </Badge>
  ) : (
    <Badge
      tone='warning'
      size='sm'
      title='Access to a rootful Docker socket is root on the host. Allowed for dev and CI only.'
    >
      rootful
    </Badge>
  );
}

export function OutdatedBadge({ worker }: { worker: Worker }) {
  if (!worker.outdated) return null;
  return (
    <Badge tone='warning' size='sm' title='The agent is older than core. Pull its image again.'>
      outdated
    </Badge>
  );
}
