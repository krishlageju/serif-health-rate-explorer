import { createContext, useContext, type ReactNode } from 'react'
import type { ApiClient } from './client'

// Holds the ApiClient for the component tree; set once in main.tsx.
const ApiContext = createContext<ApiClient | null>(null)

/** Makes an ApiClient available to every component below it (dependency injection for the UI). */
export function ApiProvider({ client, children }: { client: ApiClient; children: ReactNode }) {
  return <ApiContext.Provider value={client}>{children}</ApiContext.Provider>
}

/** Returns the ApiClient from the nearest ApiProvider. */
export function useApi(): ApiClient {
  const client = useContext(ApiContext)
  if (!client) throw new Error('useApi must be used inside <ApiProvider>')
  return client
}
