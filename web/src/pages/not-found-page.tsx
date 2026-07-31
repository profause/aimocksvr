import { Link } from 'react-router'
import { Button } from '@/components/ui/button'

export function NotFound() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center">
      <h1 className="text-6xl font-bold">404</h1>
      <p className="text-muted-foreground mt-4 text-lg">Page not found</p>
      <Button asChild className="mt-6">
        <Link to="/">Go home</Link>
      </Button>
    </div>
  )
}
