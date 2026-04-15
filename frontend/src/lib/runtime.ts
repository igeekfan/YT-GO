import {EventsOn as DesktopEventsOn} from '../../wailsjs/runtime/runtime'

export function EventsOn(eventName: string, callback: (data: any) => void) {
    if (typeof window !== 'undefined' && typeof (window as any).go?.main?.App !== 'undefined') {
        return DesktopEventsOn(eventName, callback)
    }
    return () => {}
}