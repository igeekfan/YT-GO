import {ReactNode} from 'react'
import {AuthStatus} from '../../lib/auth'
import {useI18n} from '../../i18n/context'
import LoginPage from '../LoginPage'
import {Button} from '@/components/ui/button'
import {RefreshCw} from 'lucide-react'

interface Props {
    status: AuthStatus
    onRetry: () => void
    onAuthenticated: () => void
    children: ReactNode
}

export default function AuthBoundary({status, onRetry, onAuthenticated, children}: Props) {
    const {t} = useI18n()

    if (status === 'checking') {
        return (
            <div className="min-h-screen bg-background flex items-center justify-center">
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                    <RefreshCw className="h-4 w-4 animate-spin" />
                    {t('login.verifying')}
                </div>
            </div>
        )
    }

    if (status === 'connection-error') {
        return (
            <div className="min-h-screen bg-background flex items-center justify-center px-4">
                <div className="text-center space-y-3">
                    <p className="text-sm text-destructive">{t('login.connectionError')}</p>
                    <Button onClick={onRetry}>{t('login.retry')}</Button>
                </div>
            </div>
        )
    }

    if (status === 'login-required') return <LoginPage onAuthenticated={onAuthenticated} />
    return <>{children}</>
}
