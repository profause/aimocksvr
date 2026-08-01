import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { Endpoint, CreateEndpointParams, UpdateEndpointParams } from '@/types/api'
import { createEndpoint, updateEndpoint } from '@/api/endpoints'

const endpointSchema = z.object({
  method: z.enum(['GET', 'POST', 'PUT', 'PATCH', 'DELETE']),
  path: z.string().min(1, 'Path is required').startsWith('/', 'Path must start with /'),
  description: z.string(),
  prompt: z.string().min(1, 'Prompt is required'),
  response_type: z.enum(['json', 'text', 'html']),
  stateful: z.boolean(),
  status: z.enum(['active', 'inactive', 'draft']),
  request_schema: z.string(),
  error_sim: z.string(),
  public: z.boolean(),
})

type EndpointFormValues = z.infer<typeof endpointSchema>

interface EndpointFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  endpoint?: Endpoint | null
  onSuccess: () => void
}

const defaultValues: EndpointFormValues = {
  method: 'GET',
  path: '',
  description: '',
  prompt: '',
  response_type: 'json',
  stateful: false,
  status: 'active',
  request_schema: '',
  error_sim: '',
  public: true,
}

function getFormValues(endpoint: Endpoint): EndpointFormValues {
  return {
    method: endpoint.method as EndpointFormValues['method'],
    path: endpoint.path,
    description: endpoint.description || '',
    prompt: endpoint.prompt,
    response_type: (endpoint.response_type as EndpointFormValues['response_type']) || 'json',
    stateful: endpoint.stateful,
    status: (endpoint.status as EndpointFormValues['status']) || 'active',
    request_schema: endpoint.request_schema || '',
    error_sim: endpoint.error_sim || '',
    public: endpoint.public,
  }
}

export function EndpointForm({ open, onOpenChange, endpoint, onSuccess }: EndpointFormProps) {
  const [loading, setLoading] = useState(false)
  const isEdit = !!endpoint

  const form = useForm<EndpointFormValues>({
    resolver: zodResolver(endpointSchema),
    defaultValues: isEdit ? getFormValues(endpoint!) : defaultValues,
  })

  useEffect(() => {
    if (open) {
      form.reset(isEdit ? getFormValues(endpoint!) : defaultValues)
    }
  }, [open, endpoint, form, isEdit])

  async function onSubmit(values: EndpointFormValues) {
    setLoading(true)
    try {
      if (isEdit) {
        const params: UpdateEndpointParams = {
          description: values.description || undefined,
          prompt: values.prompt,
          response_type: values.response_type,
          stateful: values.stateful,
          request_schema: values.request_schema || undefined,
          error_sim: values.error_sim || undefined,
          public: values.public,
        }
        await updateEndpoint(endpoint!.id, params)
        toast.success('Endpoint updated')
      } else {
        const params: CreateEndpointParams = {
          method: values.method,
          path: values.path,
          description: values.description || undefined,
          prompt: values.prompt,
          response_type: values.response_type,
          stateful: values.stateful,
          request_schema: values.request_schema || undefined,
          error_sim: values.error_sim || undefined,
          public: values.public,
        }
        await createEndpoint(params)
        toast.success('Endpoint created')
      }
      onSuccess()
      onOpenChange(false)
    } catch (err: unknown) {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Failed to save endpoint'
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEdit ? 'Edit Endpoint' : 'Create Endpoint'}</DialogTitle>
          <DialogDescription>
            {isEdit
              ? 'Modify the endpoint configuration. Changes create a new version.'
              : 'Define a new mock endpoint with AI-powered response generation.'}
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={form.handleSubmit(onSubmit)} className="flex flex-col gap-4">
          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="method">Method</Label>
              <Select
                value={form.watch('method')}
                onValueChange={(v) => form.setValue('method', v as EndpointFormValues['method'])}
              >
                <SelectTrigger id="method">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="GET">GET</SelectItem>
                  <SelectItem value="POST">POST</SelectItem>
                  <SelectItem value="PUT">PUT</SelectItem>
                  <SelectItem value="PATCH">PATCH</SelectItem>
                  <SelectItem value="DELETE">DELETE</SelectItem>
                </SelectContent>
              </Select>
              {form.formState.errors.method && (
                <p className="text-destructive text-xs">{form.formState.errors.method.message}</p>
              )}
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor="response_type">Response Type</Label>
              <Select
                value={form.watch('response_type')}
                onValueChange={(v) =>
                  form.setValue('response_type', v as EndpointFormValues['response_type'])
                }
              >
                <SelectTrigger id="response_type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="json">JSON</SelectItem>
                  <SelectItem value="text">Text</SelectItem>
                  <SelectItem value="html">HTML</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="path">Path</Label>
            <Input
              id="path"
              placeholder="/users/:id"
              disabled={isEdit}
              {...form.register('path')}
            />
            {form.formState.errors.path && (
              <p className="text-destructive text-xs">{form.formState.errors.path.message}</p>
            )}
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="prompt">Prompt</Label>
            <Textarea
              id="prompt"
              placeholder="Describe the behavior of this endpoint..."
              rows={3}
              {...form.register('prompt')}
            />
            {form.formState.errors.prompt && (
              <p className="text-destructive text-xs">{form.formState.errors.prompt.message}</p>
            )}
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="description">Description</Label>
            <Input
              id="description"
              placeholder="Optional description"
              {...form.register('description')}
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="status">Status</Label>
              <Select
                value={form.watch('status')}
                onValueChange={(v) =>
                  form.setValue('status', v as EndpointFormValues['status'])
                }
              >
                <SelectTrigger id="status">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="active">Active</SelectItem>
                  <SelectItem value="inactive">Inactive</SelectItem>
                  <SelectItem value="draft">Draft</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="flex flex-col gap-1">
                <Label htmlFor="stateful">Stateful</Label>
                <p className="text-muted-foreground text-xs">Persist resources</p>
              </div>
              <Switch
                id="stateful"
                checked={form.watch('stateful')}
                onCheckedChange={(v) => form.setValue('stateful', v)}
              />
            </div>
          </div>

          <div className="flex items-center justify-between rounded-lg border p-3">
            <div className="flex flex-col gap-1">
              <Label htmlFor="public">Public Endpoint</Label>
              <p className="text-muted-foreground text-xs">No authentication required</p>
            </div>
            <Switch
              id="public"
              checked={form.watch('public')}
              onCheckedChange={(v) => form.setValue('public', v)}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="request_schema">Request Schema (JSON Schema)</Label>
            <Textarea
              id="request_schema"
              placeholder='{"type":"object","properties":{"email":{"type":"string","format":"email"}}}'
              rows={4}
              className="font-mono text-xs"
              {...form.register('request_schema')}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label htmlFor="error_sim">Error Simulation</Label>
            <Textarea
              id="error_sim"
              placeholder='{"status":500,"failure_rate":30}'
              rows={3}
              className="font-mono text-xs"
              {...form.register('error_sim')}
            />
          </div>

          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? 'Saving...' : isEdit ? 'Update' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
