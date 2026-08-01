import { useState, useCallback } from 'react'
import { useMutation } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Upload, FileJson, FileCode, CheckCircle, XCircle, SkipForward } from 'lucide-react'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { MethodBadge } from '@/components/method-badge'
import { importOpenAPI, importPostman } from '@/api/imports'
import { cn } from '@/lib/utils'

interface ImportResult {
  parsed: number
  created: number
  skipped: number
  endpoints: { id: string; method: string; path: string }[]
}

function FileUpload({ importFn, label, accept }: { importFn: (file: File) => Promise<ImportResult>; label: string; accept: string }) {
  const [dragOver, setDragOver] = useState(false)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)

  const mutation = useMutation({
    mutationFn: importFn,
    onSuccess: (result) => {
      toast.success(`Imported ${result.created} endpoints (${result.skipped} skipped)`)
    },
    onError: (err: unknown) => {
      const message =
        err && typeof err === 'object' && 'message' in err
          ? (err as { message: string }).message
          : 'Import failed'
      toast.error(message)
    },
  })

  const handleFile = useCallback(
    (file: File) => {
      setSelectedFile(file)
      mutation.mutate(file)
    },
    [mutation]
  )

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setDragOver(false)
      const file = e.dataTransfer.files[0]
      if (file) handleFile(file)
    },
    [handleFile]
  )

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0]
      if (file) handleFile(file)
    },
    [handleFile]
  )

  return (
    <div className="flex flex-col gap-4">
      <div
        className={cn(
          'flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-12 transition-colors',
          dragOver
            ? 'border-primary bg-primary/5'
            : 'border-muted-foreground/25 hover:border-primary/50'
        )}
        onDragOver={(e) => {
          e.preventDefault()
          setDragOver(true)
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={handleDrop}
      >
        <Upload className="mb-4 size-10 text-muted-foreground" />
        <p className="mb-2 text-sm font-medium">
          {selectedFile ? selectedFile.name : `Drop ${label} here or click to browse`}
        </p>
        <p className="text-muted-foreground mb-4 text-xs">
          {accept}
        </p>
        <input
          type="file"
          accept={accept}
          onChange={handleChange}
          className="hidden"
          id={`file-${label}`}
        />
        <label htmlFor={`file-${label}`}>
          <Button variant="outline" asChild>
            <span>Choose File</span>
          </Button>
        </label>
      </div>

      {mutation.isPending && (
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <div className="size-4 animate-spin rounded-full border-2 border-primary border-t-transparent" />
          Importing...
        </div>
      )}

      {mutation.isSuccess && mutation.data && (
        <ImportSummary result={mutation.data} />
      )}

      {mutation.isError && (
        <div className="flex items-center gap-2 rounded-lg border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
          <XCircle className="size-4" />
          Import failed. Check the file format and try again.
        </div>
      )}
    </div>
  )
}

function ImportSummary({ result }: { result: ImportResult }) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex gap-4">
        <div className="flex items-center gap-2 rounded-lg border p-3 flex-1">
          <CheckCircle className="size-4 text-green-600" />
          <div>
            <p className="text-2xl font-bold">{result.created}</p>
            <p className="text-muted-foreground text-xs">Created</p>
          </div>
        </div>
        <div className="flex items-center gap-2 rounded-lg border p-3 flex-1">
          <SkipForward className="size-4 text-amber-600" />
          <div>
            <p className="text-2xl font-bold">{result.skipped}</p>
            <p className="text-muted-foreground text-xs">Skipped</p>
          </div>
        </div>
        <div className="flex items-center gap-2 rounded-lg border p-3 flex-1">
          <FileCode className="size-4 text-blue-600" />
          <div>
            <p className="text-2xl font-bold">{result.parsed}</p>
            <p className="text-muted-foreground text-xs">Parsed</p>
          </div>
        </div>
      </div>

      {result.endpoints.length > 0 && (
        <>
          <Separator />
          <div>
            <h4 className="mb-2 text-sm font-medium">Imported Endpoints</h4>
            <div className="max-h-64 overflow-auto rounded-md border">
              <table className="w-full text-sm">
                <thead className="bg-muted">
                  <tr>
                    <th className="px-3 py-2 text-left font-medium">Method</th>
                    <th className="px-3 py-2 text-left font-medium">Path</th>
                  </tr>
                </thead>
                <tbody>
                  {result.endpoints.map((ep) => (
                    <tr key={ep.id} className="border-t">
                      <td className="px-3 py-2">
                        <MethodBadge method={ep.method} />
                      </td>
                      <td className="px-3 py-2 font-mono text-xs">{ep.path}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </>
      )}
    </div>
  )
}

export function ImportsPage() {
  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Imports</h1>
        <p className="text-muted-foreground mt-1">
          Import OpenAPI specs or Postman collections to bootstrap your mock endpoints.
        </p>
      </div>

      <Tabs defaultValue="openapi">
        <TabsList>
          <TabsTrigger value="openapi">
            <FileJson className="mr-2 size-4" />
            OpenAPI
          </TabsTrigger>
          <TabsTrigger value="postman">
            <FileCode className="mr-2 size-4" />
            Postman
          </TabsTrigger>
        </TabsList>

        <TabsContent value="openapi" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>Import OpenAPI Specification</CardTitle>
              <CardDescription>
                Upload a Swagger 2.0 or OpenAPI 3.x document in JSON or YAML format.
                Every operation becomes a mock endpoint.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <FileUpload
                importFn={importOpenAPI}
                label="OpenAPI spec"
                accept=".json,.yaml,.yml"
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="postman" className="mt-6">
          <Card>
            <CardHeader>
              <CardTitle>Import Postman Collection</CardTitle>
              <CardDescription>
                Upload a Postman Collection v2.0/v2.1 JSON file.
                Every request becomes a mock endpoint.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <FileUpload
                importFn={importPostman}
                label="Postman collection"
                accept=".json"
              />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  )
}
