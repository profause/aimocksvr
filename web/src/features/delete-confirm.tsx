import { useState } from 'react'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { deleteEndpoint } from '@/api/endpoints'

interface DeleteConfirmProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  endpointId: string
  endpointPath: string
  onSuccess: () => void
}

export function DeleteConfirm({ open, onOpenChange, endpointId, endpointPath, onSuccess }: DeleteConfirmProps) {
  const [loading, setLoading] = useState(false)

  async function handleDelete() {
    setLoading(true)
    try {
      await deleteEndpoint(endpointId)
      toast.success('Endpoint deleted')
      onSuccess()
      onOpenChange(false)
    } catch (err: unknown) {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Failed to delete endpoint'
      toast.error(message)
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete Endpoint</DialogTitle>
          <DialogDescription>
            Are you sure you want to delete <code className="text-sm font-mono">{endpointPath}</code>?
            This action cannot be undone.
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={handleDelete} disabled={loading}>
            {loading ? 'Deleting...' : 'Delete'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
