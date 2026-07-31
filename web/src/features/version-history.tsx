import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { formatDistanceToNow } from 'date-fns'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Separator } from '@/components/ui/separator'
import { listVersions, diffVersion as fetchDiff, rollback } from '@/api/endpoints'
import type { EndpointVersion } from '@/types/api'

interface VersionHistoryProps {
  endpointId: string
  currentVersion: number
}

export function VersionHistory({ endpointId, currentVersion }: VersionHistoryProps) {
  const queryClient = useQueryClient()
  const [diffVersion, setDiffVersion] = useState<number | null>(null)
  const [diffData, setDiffData] = useState<{ field: string; from: string; to: string }[] | null>(null)
  const [loadingDiff, setLoadingDiff] = useState(false)

  const { data: versions, isLoading } = useQuery({
    queryKey: ['versions', endpointId],
    queryFn: () => listVersions(endpointId),
  })

  const rollbackMutation = useMutation({
    mutationFn: (version: number) => rollback(endpointId, version),
    onSuccess: () => {
      toast.success('Rolled back successfully')
      queryClient.invalidateQueries({ queryKey: ['endpoints'] })
      queryClient.invalidateQueries({ queryKey: ['versions', endpointId] })
    },
    onError: (err: unknown) => {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Failed to rollback'
      toast.error(message)
    },
  })

  async function handleDiff(version: number) {
    if (diffVersion === version) {
      setDiffVersion(null)
      setDiffData(null)
      return
    }
    setLoadingDiff(true)
    try {
      const data = await fetchDiff(endpointId, version)
      setDiffVersion(version)
      setDiffData(data.changes)
    } catch {
      toast.error('Failed to load diff')
    } finally {
      setLoadingDiff(false)
    }
  }

  function handleRollback(version: number) {
    rollbackMutation.mutate(version)
  }

  if (isLoading) {
    return <p className="text-muted-foreground text-sm">Loading versions...</p>
  }

  if (!versions || versions.length === 0) {
    return <p className="text-muted-foreground text-sm">No versions yet.</p>
  }

  return (
    <div className="flex flex-col gap-3">
      {versions.map((v: EndpointVersion) => (
        <div key={v.id} className="rounded-lg border p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <Badge variant={v.version === currentVersion ? 'default' : 'outline'}>
                v{v.version}
              </Badge>
              <span className="text-muted-foreground text-sm">
                {formatDistanceToNow(new Date(v.created_at))} ago
              </span>
              {v.version === currentVersion && (
                <Badge variant="secondary" className="text-xs">Current</Badge>
              )}
            </div>
            <div className="flex gap-2">
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleDiff(v.version)}
                disabled={loadingDiff || v.version === currentVersion}
              >
                {diffVersion === v.version ? 'Hide Diff' : 'Diff'}
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => handleRollback(v.version)}
                disabled={v.version === currentVersion || rollbackMutation.isPending}
              >
                Rollback
              </Button>
            </div>
          </div>

          {diffVersion === v.version && diffData && (
            <>
              <Separator className="my-3" />
              <div className="space-y-2">
                <p className="text-sm font-medium">Changes from current version:</p>
                {diffData.length === 0 ? (
                  <p className="text-muted-foreground text-sm">No changes detected.</p>
                ) : (
                  <div className="rounded-md border">
                    {diffData.map((change, idx) => (
                      <div
                        key={idx}
                        className="grid grid-cols-3 gap-2 border-b px-3 py-2 text-sm last:border-b-0"
                      >
                        <span className="font-medium">{change.field}</span>
                        <span className="text-destructive line-through">{change.from || '(empty)'}</span>
                        <span className="text-green-600 dark:text-green-400">{change.to || '(empty)'}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </>
          )}
        </div>
      ))}
    </div>
  )
}
