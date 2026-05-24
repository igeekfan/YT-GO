import React from 'react'
import {createRoot} from 'react-dom/client'
import './styles/globals.css'
import App from './App'
import ErrorBoundary from './components/ErrorBoundary'
import {I18nProvider} from './i18n/context'
import {Toaster} from '@/components/ui/sonner'
import {TooltipProvider} from '@/components/ui/tooltip'

const container = document.getElementById('root')

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <I18nProvider>
            <TooltipProvider>
                <ErrorBoundary>
                    <App/>
                </ErrorBoundary>
            </TooltipProvider>
            <Toaster position="bottom-center" />
        </I18nProvider>
    </React.StrictMode>
)
