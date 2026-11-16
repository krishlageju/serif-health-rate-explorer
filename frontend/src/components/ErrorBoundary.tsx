import { Component, type ErrorInfo, type ReactNode } from 'react'

interface State {
  error: Error | null
}

/** Catches render errors so a bug shows a message with a reload button instead of a blank page. */
export class ErrorBoundary extends Component<{ children: ReactNode }, State> {
  state: State = { error: null }

  /** Switches to the fallback view when a child throws during render. */
  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  /** Logs the error and component stack for debugging. */
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('UI error', error, info.componentStack)
  }

  /** Renders the children, or the fallback after an error. */
  render() {
    if (!this.state.error) return this.props.children
    return (
      <div className="main">
        <section className="card empty" role="alert">
          <h2>Something went wrong</h2>
          <p className="muted">{this.state.error.message}</p>
          <button type="button" className="btn btn-ghost" onClick={() => window.location.reload()}>
            Reload page
          </button>
        </section>
      </div>
    )
  }
}
