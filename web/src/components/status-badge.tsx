import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

const statusColors: Record<string, string> = {
  active: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-300',
  inactive: 'bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-300',
  draft: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-300',
}

export function StatusBadge({ status }: { status: string }) {
  return (
    <Badge variant="outline" className={cn('capitalize', statusColors[status])}>
      {status}
    </Badge>
  )
}
