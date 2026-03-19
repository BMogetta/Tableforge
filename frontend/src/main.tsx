// Telemetry must be imported before React and the app so the providers
// are registered before any fetch calls or renders happen.
import { initWebVitals, initErrorHandler } from './lib/telemetry'
initErrorHandler()
initWebVitals()

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { QueryClientProvider } from '@tanstack/react-query'
import { routeTree } from './routeTree.gen'
import { TanStackDevtools } from '@tanstack/react-devtools'
import { ReactQueryDevtoolsPanel } from '@tanstack/react-query-devtools'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { pacerDevtoolsPlugin } from '@tanstack/react-pacer-devtools'
//import { TanStackRouterDevtoolsInProd } from '@tanstack/react-router-devtools'
import { queryClient } from './lib/queryClient'
import './styles/global.css'

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      <TanStackDevtools
        plugins={[
          {
            name: 'TanStack Query',
            render: <ReactQueryDevtoolsPanel />,
          },
          {
            name: 'TanStack Router',
            render: <TanStackRouterDevtools router={router} />,
          },
          pacerDevtoolsPlugin(),
        ]}
      ></TanStackDevtools>
    </QueryClientProvider>
  </StrictMode>,
)
