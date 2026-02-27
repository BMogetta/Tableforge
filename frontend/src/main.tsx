// Telemetry must be imported before React and the app so the providers
// are registered before any fetch calls or renders happen.
import { initWebVitals, initErrorHandler } from './telemetry'
initErrorHandler()
initWebVitals()

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import './styles/global.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
      <App />
  </StrictMode>,
)