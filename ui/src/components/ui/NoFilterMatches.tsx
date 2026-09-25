import { ListFilter, X } from 'lucide-react';
import { Button } from './Button';
import { EmptyState } from './EmptyState';

/** The empty state for a list whose filters exclude everything, with the way out. */
export function NoFilterMatches({ noun, onReset }: { noun: string; onReset: () => void }) {
  return (
    <div className='py-16'>
      <EmptyState
        icon={<ListFilter className='h-5 w-5' />}
        title={`No ${noun} match these filters`}
        description='Drop one of the chips above, or clear them all and start again.'
        action={
          <Button size='sm' onClick={onReset}>
            <X className='h-3.5 w-3.5' /> Reset filters
          </Button>
        }
      />
    </div>
  );
}
