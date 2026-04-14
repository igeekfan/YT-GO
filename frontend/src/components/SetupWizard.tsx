import {useState, useEffect} from 'react'
import {SelectFolder, SelectCookiesFile, CheckYtDlp} from '../../wailsjs/go/main/App'
import {useI18n} from '../i18n/context'

interface Props {
    onComplete: (outputDir: string, cookiesFrom: string, cookiesFile: string, proxy: string) => void
}

export default function SetupWizard({onComplete}: Props) {
    const {t, lang} = useI18n()
    const [step, setStep] = useState(1)
    const [outputDir, setOutputDir] = useState('')
    const [cookiesFrom, setCookiesFrom] = useState('')
    const [cookiesFile, setCookiesFile] = useState('')
    const [proxy, setProxy] = useState('')
    const [ytdlpOk, setYtdlpOk] = useState(false)

    useEffect(() => {
        CheckYtDlp().then(status => {
            setYtdlpOk(status.available)
        }).catch(() => {})
    }, [])

    const handleSelectFolder = async () => {
        const dir = await SelectFolder()
        if (dir) setOutputDir(dir)
    }

    const handleSelectCookiesFile = async () => {
        const file = await SelectCookiesFile()
        if (file) setCookiesFile(file)
    }

    const handleComplete = () => {
        onComplete(outputDir, cookiesFrom, cookiesFile, proxy)
    }

    const totalSteps = 2

    return (
        <div className="setup-wizard-overlay">
            <div className="setup-wizard">
                <div className="setup-wizard-header">
                    <div className="setup-wizard-title">
                        {step === 1 
                            ? t('setup.step1Title')
                            : t('setup.step2Title')
                        }
                    </div>
                    <div className="setup-wizard-progress">
                        {step} / {totalSteps}
                    </div>
                </div>

                <div className="setup-wizard-content">
                    {step === 1 && (
                        <div className="setup-step">
                            <p className="setup-desc">
                                {t('setup.selectDir')}
                            </p>
                            <div className="setup-input-group">
                                <input
                                    type="text"
                                    className="setup-input"
                                    value={outputDir}
                                    onChange={e => setOutputDir(e.target.value)}
                                    placeholder={t('setup.selectDirPlaceholder')}
                                    readOnly
                                />
                                <button className="btn-secondary" onClick={handleSelectFolder}>
                                    {t('outputDir.browse')}
                                </button>
                            </div>
                            {ytdlpOk && (
                                <div className="setup-ytdlp-ok">
                                    ✓ {t('setup.ytReady')}
                                </div>
                            )}
                        </div>
                    )}

                    {step === 2 && (
                        <div className="setup-step">
                            <p className="setup-desc">
                                {t('setup.cookiesDesc')}
                            </p>
                            
                            {/* Option 1: Import from browser */}
                            <div className="setup-field">
                                <label className="setup-label">{t('settings.cookiesFrom')}</label>
                                <select
                                    className="setup-select"
                                    value={cookiesFrom}
                                    onChange={e => {
                                        setCookiesFrom(e.target.value)
                                        if (e.target.value) setCookiesFile('')
                                    }}
                                >
                                    <option value="">{t('settings.cookiesFromNone')}</option>
                                    <option value="chrome">Chrome</option>
                                    <option value="firefox">Firefox</option>
                                    <option value="edge">Edge</option>
                                    <option value="opera">Opera</option>
                                    <option value="brave">Brave</option>
                                    <option value="vivaldi">Vivaldi</option>
                                    <option value="safari">Safari</option>
                                </select>
                                <p className="setup-hint">{t('settings.cookiesFromHint')}</p>
                            </div>

                            {/* Divider with "or" */}
                            <div className="setup-divider">
                                <span>{t('setup.or')}</span>
                            </div>

                            {/* Option 2: Import from file */}
                            <div className="setup-field">
                                <label className="setup-label">{t('settings.cookiesFile')}</label>
                                <div className="setup-input-group">
                                    <input
                                        type="text"
                                        className="setup-input"
                                        value={cookiesFile}
                                        onChange={e => {
                                            setCookiesFile(e.target.value)
                                            if (e.target.value) setCookiesFrom('')
                                        }}
                                        placeholder={t('settings.cookiesFilePlaceholder')}
                                        disabled={!!cookiesFrom}
                                        readOnly
                                    />
                                    <button 
                                        className="btn-secondary" 
                                        onClick={handleSelectCookiesFile}
                                        disabled={!!cookiesFrom}
                                    >
                                        {t('outputDir.browse')}
                                    </button>
                                </div>
                                <p className="setup-hint">{t('settings.cookiesFileHint')}</p>
                                
                                {/* How to export instructions */}
                                <div className="setup-howto">
                                    <details>
                                        <summary>{t('settings.cookiesFileHowTo')}</summary>
                                        <div className="setup-howto-content">
                                            <p>{t('setup.howtoStep1')}:</p>
                                            <ul>
                                                <li>{t('setup.howtoChrome')}</li>
                                                <li>{t('setup.howtoFirefox')}</li>
                                            </ul>
                                            <p>{t('setup.howtoStep2')}</p>
                                            <p>{t('setup.howtoStep3')}</p>
                                        </div>
                                    </details>
                                </div>
                            </div>

                            {/* Proxy */}
                            <div className="setup-divider" style={{marginTop: 24}} />
                            <div className="setup-field">
                                <label className="setup-label">{t('setup.proxy')}</label>
                                <input
                                    type="text"
                                    className="setup-input"
                                    value={proxy}
                                    onChange={e => setProxy(e.target.value)}
                                    placeholder="http://127.0.0.1:7890"
                                />
                                <p className="setup-hint">{t('settings.proxyHint')}</p>
                            </div>
                        </div>
                    )}
                </div>

                <div className="setup-wizard-footer">
                    {step > 1 && (
                        <button className="btn-secondary" onClick={() => setStep(step - 1)}>
                            {t('setup.back')}
                        </button>
                    )}
                    {step < totalSteps ? (
                        <button 
                            className="btn-primary" 
                            onClick={() => setStep(step + 1)}
                            disabled={step === 1 && !outputDir}
                        >
                            {t('setup.next')}
                        </button>
                    ) : (
                        <button 
                            className="btn-primary" 
                            onClick={handleComplete}
                            disabled={!outputDir}
                        >
                            {t('setup.done')}
                        </button>
                    )}
                </div>
            </div>
        </div>
    )
}