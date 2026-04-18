// If this window was opened as an OAuth popup, close it.
// The opener polls auth.me() to detect when login succeeded.
// Don't close on error redirects so the user can see the error.
if (window.opener && !window.location.search.includes('error')) {
  window.close()
}

// Telemetry must be imported before React and the app so the providers
// are registered before any fetch calls or renders happen.
import { initConsoleBridge, initErrorHandler, initWebVitals } from './lib/telemetry'

initErrorHandler()
initConsoleBridge()
initWebVitals()

import { TanStackDevtools } from '@tanstack/react-devtools'
import { pacerDevtoolsPlugin } from '@tanstack/react-pacer-devtools'
import { QueryClientProvider } from '@tanstack/react-query'
import { ReactQueryDevtoolsPanel } from '@tanstack/react-query-devtools'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { TanStackRouterDevtools } from '@tanstack/react-router-devtools'
import { FlagProvider } from '@unleash/proxy-client-react'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { flagsConfig } from './lib/flags'
import { queryClient } from './lib/queryClient'
import { routeTree } from './routeTree.gen'
import './lib/i18n'
import './styles/fonts.css'
import './styles/global.css'
import { AdminDevtoolsGate } from './features/devtools/AdminDevtoolsGate'
import { ScenarioPicker } from './features/devtools/ScenarioPicker'
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

import { isDev } from '@/lib/env'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <FlagProvider config={flagsConfig}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
        {isDev ? (
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
                {
                  name: 'Scenarios',
                  render: <ScenarioPicker />,
                },
              ]}
            ></TanStackDevtools>
          </>
        ) : (
          // Prod build: owners can unlock devtools via the
          // devtools-for-admins flag. The panel (including router devtools)
          // is lazy-loaded so other users never download it.
          <AdminDevtoolsGate />
        )}
      </QueryClientProvider>
    </FlagProvider>
  </StrictMode>,
)
