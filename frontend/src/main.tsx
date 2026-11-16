import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { ApiProvider } from './api/ApiContext'
import { createApiClient } from './api/client'
import App from './App'
import { ErrorBoundary } from './components/ErrorBoundary'
import './styles.css'

// The one place that decides which API the UI talks to.
const apiClient = createApiClient(import.meta.env.VITE_API_BASE_URL ?? '')

const queryClient = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false } },
})

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <ApiProvider client={apiClient}>
        <QueryClientProvider client={queryClient}>
          <App />
        </QueryClientProvider>
      </ApiProvider>
    </ErrorBoundary>
  </StrictMode>,
)
