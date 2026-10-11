import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { Toaster } from '@/components/ui/sonner'
import { TooltipProvider } from '@/components/ui/tooltip'
import App from './App'
import './index.css'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <TooltipProvider delayDuration={0}>
      <BrowserRouter>
        <App />
        <Toaster richColors position="top-center" />
      </BrowserRouter>
    </TooltipProvider>
  </StrictMode>,
)
