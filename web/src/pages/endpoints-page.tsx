import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { MethodBadge } from '@/components/method-badge'
import { StatusBadge } from '@/components/status-badge'
import { EndpointForm } from '@/features/endpoint-form'
import { EndpointDetail } from '@/features/endpoint-detail'
import { DeleteConfirm } from '@/features/delete-confirm'
import { listEndpoints } from '@/api/endpoints'

export function EndpointsPage() {
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const limit = 20
  const [createOpen, setCreateOpen] = useState(false)
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; path: string } | null>(null)
  const [search, setSearch] = useState('')

  const { data, isLoading } = useQuery({
    queryKey: ['endpoints', page, limit],
    queryFn: () => listEndpoints(page, limit),
  })

  const totalPages = data ? Math.ceil(data.total / limit) : 0
  const filteredEndpoints =
    Array.isArray(data?.endpoints)
      ? data!.endpoints.filter(
          (e) =>
            e.path.toLowerCase().includes(search.toLowerCase()) ||
            e.method.toLowerCase().includes(search.toLowerCase()) ||
            e.description?.toLowerCase().includes(search.toLowerCase())
        )
      : []

  if (selectedId) {
    return (
      <EndpointDetail
        endpointId={selectedId}
        onBack={() => {
          setSelectedId(null)
          queryClient.invalidateQueries({ queryKey: ['endpoints'] })
        }}
      />
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Endpoints</h1>
          <p className="text-muted-foreground mt-1">
            Create and manage your mock API endpoints.
          </p>
        </div>
        <Button onClick={() => setCreateOpen(true)}>
          New Endpoint
        </Button>
      </div>

      <div className="flex items-center gap-4">
        <Input
          placeholder="Search by path, method, or description..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="max-w-sm"
        />
      </div>

      <div className="rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead className="w-24">Method</TableHead>
              <TableHead>Path</TableHead>
              <TableHead className="w-28">Status</TableHead>
              <TableHead className="w-24">Stateful</TableHead>
              <TableHead className="w-24">Public</TableHead>
              <TableHead className="w-32">Updated</TableHead>
              <TableHead className="w-40 text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading && (
              <TableRow>
                <TableCell colSpan={7} className="text-center text-muted-foreground">
                  Loading...
                </TableCell>
              </TableRow>
            )}
            {!isLoading && filteredEndpoints.length === 0 && (
              <TableRow>
                <TableCell colSpan={7} className="text-center text-muted-foreground">
                  {data && data.total === 0
                    ? 'No endpoints yet. Create your first one.'
                    : 'No endpoints match your search.'}
                </TableCell>
              </TableRow>
            )}
            {filteredEndpoints.map((endpoint) => (
              <TableRow
                key={endpoint.id}
                className="cursor-pointer"
                onClick={() => setSelectedId(endpoint.id)}
              >
                <TableCell>
                  <MethodBadge method={endpoint.method} />
                </TableCell>
                <TableCell className="font-mono text-sm">{endpoint.path}</TableCell>
                <TableCell>
                  <StatusBadge status={endpoint.status} />
                </TableCell>
                <TableCell>{endpoint.stateful ? 'Yes' : 'No'}</TableCell>
                <TableCell>{endpoint.public ? 'Yes' : 'No'}</TableCell>
                <TableCell className="text-muted-foreground text-xs">
                  {new Date(endpoint.updated_at).toLocaleDateString()}
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-2" onClick={(e) => e.stopPropagation()}>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setSelectedId(endpoint.id)}
                    >
                      View
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="text-destructive"
                      onClick={() => setDeleteTarget({ id: endpoint.id, path: endpoint.path })}
                    >
                      Delete
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <p className="text-muted-foreground text-sm">
            Page {page} of {totalPages} ({data?.total} total)
          </p>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </Button>
          </div>
        </div>
      )}

      <EndpointForm
        open={createOpen}
        onOpenChange={setCreateOpen}
        endpoint={null}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ['endpoints'] })
        }}
      />

      {deleteTarget && (
        <DeleteConfirm
          open={!!deleteTarget}
          onOpenChange={() => setDeleteTarget(null)}
          endpointId={deleteTarget.id}
          endpointPath={deleteTarget.path}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ['endpoints'] })
          }}
        />
      )}
    </div>
  )
}
