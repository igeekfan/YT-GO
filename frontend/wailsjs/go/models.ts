export namespace desktop {
	
	export class AboutInfo {
	    appVersion: string;
	    systemVersion: string;
	    githubRepo: string;
	    githubUrl: string;
	    authorEmail: string;
	
	    static createFrom(source: any = {}) {
	        return new AboutInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appVersion = source["appVersion"];
	        this.systemVersion = source["systemVersion"];
	        this.githubRepo = source["githubRepo"];
	        this.githubUrl = source["githubUrl"];
	        this.authorEmail = source["authorEmail"];
	    }
	}
	export class DiagnosticInfo {
	    ytdlpPath: string;
	    ytdlpVersion: string;
	    ytdlpFound: boolean;
	    ffmpegPath: string;
	    ffmpegVersion: string;
	    ffmpegFound: boolean;
	    nodeVersion: string;
	    appVersion: string;
	    testOutput: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new DiagnosticInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ytdlpPath = source["ytdlpPath"];
	        this.ytdlpVersion = source["ytdlpVersion"];
	        this.ytdlpFound = source["ytdlpFound"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffmpegVersion = source["ffmpegVersion"];
	        this.ffmpegFound = source["ffmpegFound"];
	        this.nodeVersion = source["nodeVersion"];
	        this.appVersion = source["appVersion"];
	        this.testOutput = source["testOutput"];
	        this.error = source["error"];
	    }
	}
	export class DownloadOptions {
	    saveDescription?: boolean;
	    saveThumbnail?: boolean;
	    embedChapters?: boolean;
	    writeSubtitles?: boolean;
	    subtitleLangs: string;
	    embedSubtitles?: boolean;
	    sponsorBlock?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DownloadOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.saveDescription = source["saveDescription"];
	        this.saveThumbnail = source["saveThumbnail"];
	        this.embedChapters = source["embedChapters"];
	        this.writeSubtitles = source["writeSubtitles"];
	        this.subtitleLangs = source["subtitleLangs"];
	        this.embedSubtitles = source["embedSubtitles"];
	        this.sponsorBlock = source["sponsorBlock"];
	    }
	}
	export class SubtitleLang {
	    code: string;
	    name: string;
	    auto: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SubtitleLang(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.name = source["name"];
	        this.auto = source["auto"];
	    }
	}
	export class VideoInfo {
	    url: string;
	    id: string;
	    title: string;
	    thumbnail: string;
	    duration: number;
	    uploader: string;
	    platform: string;
	    subtitles: SubtitleLang[];
	
	    static createFrom(source: any = {}) {
	        return new VideoInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.id = source["id"];
	        this.title = source["title"];
	        this.thumbnail = source["thumbnail"];
	        this.duration = source["duration"];
	        this.uploader = source["uploader"];
	        this.platform = source["platform"];
	        this.subtitles = this.convertValues(source["subtitles"], SubtitleLang);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DownloadRequest {
	    url: string;
	    outputDir: string;
	    quality: string;
	    videoInfo?: VideoInfo;
	    options?: DownloadOptions;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.outputDir = source["outputDir"];
	        this.quality = source["quality"];
	        this.videoInfo = this.convertValues(source["videoInfo"], VideoInfo);
	        this.options = this.convertValues(source["options"], DownloadOptions);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DownloadTask {
	    id: string;
	    url: string;
	    title: string;
	    thumbnail: string;
	    quality: string;
	    status: string;
	    progress: number;
	    speed: string;
	    eta: string;
	    size: string;
	    outputPath: string;
	    outputDir: string;
	    error: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new DownloadTask(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.thumbnail = source["thumbnail"];
	        this.quality = source["quality"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.speed = source["speed"];
	        this.eta = source["eta"];
	        this.size = source["size"];
	        this.outputPath = source["outputPath"];
	        this.outputDir = source["outputDir"];
	        this.error = source["error"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class Format {
	    formatId: string;
	    ext: string;
	    resolution: string;
	    fps: number;
	    vcodec: string;
	    acodec: string;
	    filesize: number;
	    tbr: number;
	    note: string;
	    hasVideo: boolean;
	    hasAudio: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Format(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.formatId = source["formatId"];
	        this.ext = source["ext"];
	        this.resolution = source["resolution"];
	        this.fps = source["fps"];
	        this.vcodec = source["vcodec"];
	        this.acodec = source["acodec"];
	        this.filesize = source["filesize"];
	        this.tbr = source["tbr"];
	        this.note = source["note"];
	        this.hasVideo = source["hasVideo"];
	        this.hasAudio = source["hasAudio"];
	    }
	}
	export class FormatInfo {
	    url: string;
	    title: string;
	    formats: Format[];
	
	    static createFrom(source: any = {}) {
	        return new FormatInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.formats = this.convertValues(source["formats"], Format);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PlaylistInfo {
	    url: string;
	    kind: string;
	    title: string;
	    uploader: string;
	    count: number;
	    videos: VideoInfo[];
	
	    static createFrom(source: any = {}) {
	        return new PlaylistInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.uploader = source["uploader"];
	        this.count = source["count"];
	        this.videos = this.convertValues(source["videos"], VideoInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Settings {
	    outputDir: string;
	    quality: string;
	    language: string;
	    theme: string;
	    proxy: string;
	    rateLimit: string;
	    maxConcurrent: number;
	    notifications: boolean;
	    saveDescription: boolean;
	    saveThumbnail: boolean;
	    writeSubtitles: boolean;
	    subtitleLangs: string;
	    embedSubtitles: boolean;
	    embedChapters: boolean;
	    sponsorBlock: boolean;
	    filenameTemplate: string;
	    mergeOutputFormat: string;
	    audioFormat: string;
	    cookiesFrom: string;
	    cookiesFile: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.outputDir = source["outputDir"];
	        this.quality = source["quality"];
	        this.language = source["language"];
	        this.theme = source["theme"];
	        this.proxy = source["proxy"];
	        this.rateLimit = source["rateLimit"];
	        this.maxConcurrent = source["maxConcurrent"];
	        this.notifications = source["notifications"];
	        this.saveDescription = source["saveDescription"];
	        this.saveThumbnail = source["saveThumbnail"];
	        this.writeSubtitles = source["writeSubtitles"];
	        this.subtitleLangs = source["subtitleLangs"];
	        this.embedSubtitles = source["embedSubtitles"];
	        this.embedChapters = source["embedChapters"];
	        this.sponsorBlock = source["sponsorBlock"];
	        this.filenameTemplate = source["filenameTemplate"];
	        this.mergeOutputFormat = source["mergeOutputFormat"];
	        this.audioFormat = source["audioFormat"];
	        this.cookiesFrom = source["cookiesFrom"];
	        this.cookiesFile = source["cookiesFile"];
	    }
	}
	
	export class UpdateInfo {
	    hasUpdate: boolean;
	    currentVersion: string;
	    latestVersion: string;
	    releaseName: string;
	    releaseBody: string;
	    htmlUrl: string;
	    publishedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.releaseName = source["releaseName"];
	        this.releaseBody = source["releaseBody"];
	        this.htmlUrl = source["htmlUrl"];
	        this.publishedAt = source["publishedAt"];
	    }
	}
	
	export class YtDlpStatus {
	    available: boolean;
	    version: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new YtDlpStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.version = source["version"];
	        this.path = source["path"];
	    }
	}

}

