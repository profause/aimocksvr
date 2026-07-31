import { useQuery } from '@tanstack/react-query'
import { formatDistanceToNow } from 'date-fns'

import { Badge } from '@/components/ui/badge'
import { listHistory } from '@/api/endpoints'
import type { RequestHistory } from '@/types/api'

interface RequestHistoryProps {
  endpointId: string
}

export function RequestHistoryView({ endpointId }: RequestHistoryProps) {
  const { data: history, isLoading } = useQuery({
    queryKey: ['history', endpointId],
    queryFn: () => listHistory(endpointId),
  })

  if (isLoading) {
    return <p className="text-muted-foreground text-sm">Loading history...</p>
  }

  if (!history || history.length === 0) {
    return <p className="text-muted-foreground text-sm">No requests recorded yet.</p>
  }

  return (
    <div className="flex flex-col gap-3">
      {history.map((req: RequestHistory) => (
        <div key={req.id} className="rounded-lg border p-4">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-muted-foreground text-xs">
              {formatDistanceToNow(new Date(req.created_at))} ago
            </span>
            <Badge variant="secondary" className="text-xs">
              {req.latency}ms
            </Badge>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <p className="mb-1 text-xs font-medium text-muted-foreground">Request</p>
              <pre className="max-h-40 overflow-auto rounded bg-muted p-2 text-xs">
                {JSON.stringify(req.request, null, 2)}
              </pre>
            </div>
            <div>
              <p className="mb-1 text-xs font-medium text-muted-foreground">Response</p>
              <pre className="max-h-40 overflow-auto rounded bg-muted p-2 text-xs">
                {JSON.stringify(req.response, null, 2)}
              </pre>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}
