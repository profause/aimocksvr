import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { ArrowLeft, Copy } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Separator } from '@/components/ui/separator'
import { MethodBadge } from '@/components/method-badge'
import { StatusBadge } from '@/components/status-badge'
import { VersionHistory } from '@/features/version-history'
import { RequestHistoryView } from '@/features/request-history'
import { EndpointForm } from '@/features/endpoint-form'
import { getEndpoint } from '@/api/endpoints'

interface EndpointDetailProps {
  endpointId: string
  onBack: () => void
}

export function EndpointDetail({ endpointId, onBack }: EndpointDetailProps) {
  const queryClient = useQueryClient()
  const [editOpen, setEditOpen] = useState(false)

  const { data: endpoint, isLoading } = useQuery({
    queryKey: ['endpoint', endpointId],
    queryFn: () => getEndpoint(endpointId),
  })

  const copyUrl = () => {
    if (!endpoint) return
    const url = `${window.location.origin}${endpoint.path}`
    navigator.clipboard.writeText(url)
    toast.success('URL copied to clipboard')
  }

  if (isLoading) {
    return <p className="text-muted-foreground">Loading endpoint...</p>
  }

  if (!endpoint) {
    return <p className="text-destructive">Endpoint not found.</p>
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-4">
        <Button variant="ghost" size="icon" onClick={onBack}>
          <ArrowLeft className="size-4" />
        </Button>
        <div className="flex flex-1 items-center gap-3">
          <MethodBadge method={endpoint.method} />
          <code className="rounded bg-muted px-2 py-1 text-sm font-mono">{endpoint.path}</code>
          <StatusBadge status={endpoint.status} />
          {endpoint.stateful && <Badge variant="secondary">Stateful</Badge>}
          {!endpoint.public && <Badge variant="outline">Private</Badge>}
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={copyUrl}>
            <Copy className="size-3" />
            Copy URL
          </Button>
          <Button size="sm" onClick={() => setEditOpen(true)}>
            Edit
          </Button>
        </div>
      </div>

      {endpoint.description && (
        <p className="text-muted-foreground">{endpoint.description}</p>
      )}

      <div className="rounded-lg border p-4">
        <h3 className="mb-2 text-sm font-medium">Prompt</h3>
        <p className="whitespace-pre-wrap text-sm">{endpoint.prompt}</p>
      </div>

      {endpoint.request_schema && (
        <div className="rounded-lg border p-4">
          <h3 className="mb-2 text-sm font-medium">Request Schema</h3>
          <pre className="max-h-60 overflow-auto rounded bg-muted p-3 text-xs">
            {JSON.stringify(JSON.parse(endpoint.request_schema), null, 2)}
          </pre>
        </div>
      )}

      {endpoint.error_sim && (
        <div className="rounded-lg border p-4">
          <h3 className="mb-2 text-sm font-medium">Error Simulation</h3>
          <pre className="rounded bg-muted p-3 text-xs">
            {JSON.stringify(JSON.parse(endpoint.error_sim), null, 2)}
          </pre>
        </div>
      )}

      <Separator />

      <Tabs defaultValue="versions">
        <TabsList>
          <TabsTrigger value="versions">Versions</TabsTrigger>
          <TabsTrigger value="history">Request History</TabsTrigger>
        </TabsList>
        <TabsContent value="versions" className="mt-4">
          <VersionHistory
            endpointId={endpointId}
            currentVersion={endpoint.version ?? 1}
          />
        </TabsContent>
        <TabsContent value="history" className="mt-4">
          <RequestHistoryView endpointId={endpointId} />
        </TabsContent>
      </Tabs>

      <EndpointForm
        open={editOpen}
        onOpenChange={setEditOpen}
        endpoint={endpoint}
        onSuccess={() => {
          queryClient.invalidateQueries({ queryKey: ['endpoint', endpointId] })
          queryClient.invalidateQueries({ queryKey: ['endpoints'] })
        }}
      />
    </div>
  )
}
