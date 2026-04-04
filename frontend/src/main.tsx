// If this window was opened as an OAuth popup, close it.
// The opener polls auth.me() to detect when login succeeded.
// Don't close on error redirects so the user can see the error.
if (window.opener && !window.location.search.includes('error')) {
  window.close()
}

// Telemetry must be imported before React and the app so the providers
// are registered before any fetch calls or renders happen.
import { initWebVitals, initErrorHandler, initConsoleBridge } from './lib/telemetry'
initErrorHandler()
initConsoleBridge()
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
import './lib/i18n'
import './styles/fonts.css'
import './styles/global.css'
import { useWsDevtools } from './features/devtools/useWsDevtools'
import { WsDevtools } from './features/devtools/WsDevtools'

const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

function DevtoolsCapture() {
  useWsDevtools()
  return null
}

const isDev = import.meta.env.DEV || import.meta.env.VITE_TEST_MODE === 'true'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
      {isDev && (
        <>
          <DevtoolsCapture />
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
              {
                name: 'WebSocket',
                render: <WsDevtools />,
              },
            ]}
          ></TanStackDevtools>
        </>
      )}
    </QueryClientProvider>
  </StrictMode>,
)
