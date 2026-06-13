import {useState} from 'react'
import {setAuthToken} from '../lib/backend'
import {useI18n} from '../i18n/context'
import {Button} from '@/components/ui/button'
import {Input} from '@/components/ui/input'
import {Lock} from 'lucide-react'

interface Props {
    onAuthenticated: () => void
}

export default function LoginPage({onAuthenticated}: Props) {
    const {t} = useI18n()
    const [token, setToken] = useState('')
    const [error, setError] = useState('')
    const [loading, setLoading] = useState(false)

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault()
        if (!token.trim()) return
        setLoading(true)
        setError('')
        try {
            setAuthToken(token.trim())
            // Verify token by calling a protected endpoint.
            const base = (import.meta.env.VITE_API_BASE || '').replace(/\/$/, '')
            const resp = await fetch(`${base}/api/health`, {
                headers: {Authorization: `Bearer ${token.trim()}`},
            })
            if (resp.ok) {
                onAuthenticated()
            } else {
                setAuthToken(null)
                setError(t('login.invalidToken'))
            }
        } catch {
            setAuthToken(null)
            setError(t('login.connectionError'))
        } finally {
            setLoading(false)
        }
    }

    return (
        <div className="min-h-screen bg-background flex items-center justify-center px-4">
            <div className="w-full max-w-sm space-y-6">
                <div className="text-center space-y-2">
                    <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-primary/10 text-primary">
                        <Lock className="h-6 w-6" />
                    </div>
                    <h1 className="text-xl font-bold tracking-tight">{t('login.title')}</h1>
                    <p className="text-sm text-muted-foreground">{t('login.description')}</p>
                </div>
                <form onSubmit={handleSubmit} className="space-y-3">
                    <Input
                        type="password"
                        value={token}
                        onChange={e => { setToken(e.target.value); setError('') }}
                        placeholder={t('login.tokenPlaceholder')}
                        autoFocus
                        disabled={loading}
                    />
                    {error && <p className="text-xs text-destructive">{error}</p>}
                    <Button type="submit" className="w-full" disabled={loading || !token.trim()}>
                        {loading ? t('login.verifying') : t('login.submit')}
                    </Button>
                </form>
            </div>
        </div>
    )
}
