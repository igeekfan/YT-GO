import {useState, useEffect} from 'react'
import {SelectFolder, GetDiagnosticInfo, CheckYtDlp} from '../../wailsjs/go/main/App'
import {useI18n} from '../i18n/context'

interface Props {
    onComplete: (outputDir: string, cookiesFrom: string, proxy: string) => void
}

export default function SetupWizard({onComplete}: Props) {
    const {t, lang} = useI18n()
    const [step, setStep] = useState(1)
    const [outputDir, setOutputDir] = useState('')
    const [cookiesFrom, setCookiesFrom] = useState('')
    const [proxy, setProxy] = useState('')
    const [checking, setChecking] = useState(false)
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

    const handleComplete = () => {
        onComplete(outputDir, cookiesFrom, proxy)
    }

    const totalSteps = 2

    return (
        <div className="setup-wizard-overlay">
            <div className="setup-wizard">
                <div className="setup-wizard-header">
                    <div className="setup-wizard-title">
                        {step === 1 && (lang === 'zh-CN' ? '设置下载目录' : 'Set Download Directory')}
                        {step === 2 && (lang === 'zh-CN' ? '配置 Cookies（可选）' : 'Configure Cookies (Optional)')}
                    </div>
                    <div className="setup-wizard-progress">
                        {step} / {totalSteps}
                    </div>
                </div>

                <div className="setup-wizard-content">
                    {step === 1 && (
                        <div className="setup-step">
                            <p className="setup-desc">
                                {lang === 'zh-CN' 
                                    ? '请选择视频保存目录'
                                    : 'Please select where to save downloaded videos'}
                            </p>
                            <div className="setup-input-group">
                                <input
                                    type="text"
                                    className="setup-input"
                                    value={outputDir}
                                    onChange={e => setOutputDir(e.target.value)}
                                    placeholder={lang === 'zh-CN' ? '点击右侧浏览按钮选择...' : 'Click Browse to select...'}
                                    readOnly
                                />
                                <button className="btn-secondary" onClick={handleSelectFolder}>
                                    {t('outputDir.browse')}
                                </button>
                            </div>
                            {ytdlpOk && (
                                <div className="setup-ytdlp-ok">
                                    ✓ yt-dlp {lang === 'zh-CN' ? '已就绪' : 'ready'}
                                </div>
                            )}
                        </div>
                    )}

                    {step === 2 && (
                        <div className="setup-step">
                            <p className="setup-desc">
                                {lang === 'zh-CN'
                                    ? 'YouTube 可能需要登录验证。如果下载时遇到问题，请配置 Cookies。'
                                    : 'YouTube may require sign-in. If downloads fail, configure cookies.'}
                            </p>
                            
                            <div className="setup-field">
                                <label className="setup-label">{t('settings.cookiesFrom')}</label>
                                <select
                                    className="setup-select"
                                    value={cookiesFrom}
                                    onChange={e => setCookiesFrom(e.target.value)}
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
                                <p className="setup-hint">
                                    {lang === 'zh-CN'
                                        ? '推荐：从浏览器直接读取 Cookies，无需导出文件'
                                        : 'Recommended: Read directly from browser, no file export needed'}
                                </p>
                            </div>

                            <div className="setup-field">
                                <label className="setup-label">{t('settings.proxy')}</label>
                                <input
                                    type="text"
                                    className="setup-input"
                                    value={proxy}
                                    onChange={e => setProxy(e.target.value)}
                                    placeholder="http://127.0.0.1:7890"
                                />
                                <p className="setup-hint">
                                    {lang === 'zh-CN'
                                        ? '如果无法直接访问 YouTube，请配置代理'
                                        : 'Configure proxy if you cannot access YouTube directly'}
                                </p>
                            </div>
                        </div>
                    )}
                </div>

                <div className="setup-wizard-footer">
                    {step > 1 && (
                        <button className="btn-secondary" onClick={() => setStep(step - 1)}>
                            {lang === 'zh-CN' ? '上一步' : 'Back'}
                        </button>
                    )}
                    {step < totalSteps ? (
                        <button 
                            className="btn-primary" 
                            onClick={() => setStep(step + 1)}
                            disabled={step === 1 && !outputDir}
                        >
                            {lang === 'zh-CN' ? '下一步' : 'Next'}
                        </button>
                    ) : (
                        <button 
                            className="btn-primary" 
                            onClick={handleComplete}
                            disabled={!outputDir}
                        >
                            {lang === 'zh-CN' ? '完成' : 'Done'}
                        </button>
                    )}
                </div>
            </div>
        </div>
    )
}