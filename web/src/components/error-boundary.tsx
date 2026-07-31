import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('ErrorBoundary caught:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return (
        this.props.fallback ?? (
          <div className="flex min-h-screen items-center justify-center">
            <div className="text-center">
              <h1 className="text-2xl font-bold text-destructive">
                Something went wrong
              </h1>
              <p className="text-muted-foreground mt-2">
                {this.state.error?.message}
              </p>
              <button
                className="mt-4 rounded-md bg-primary px-4 py-2 text-primary-foreground"
                onClick={() => this.setState({ hasError: false, error: null })}
              >
                Try again
              </button>
            </div>
          </div>
        )
      )
    }

    return this.props.children
  }
}
