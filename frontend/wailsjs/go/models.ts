export namespace main {
	
	export class DiagnosticInfo {
	    ytdlpPath: string;
	    ytdlpVersion: string;
	    ytdlpFound: boolean;
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
	        this.testOutput = source["testOutput"];
	        this.error = source["error"];
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
	    }
	}
	export class DownloadRequest {
	    url: string;
	    outputDir: string;
	    quality: string;
	    videoInfo?: VideoInfo;
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.outputDir = source["outputDir"];
	        this.quality = source["quality"];
	        this.videoInfo = this.convertValues(source["videoInfo"], VideoInfo);
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
	        this.cookiesFrom = source["cookiesFrom"];
	        this.cookiesFile = source["cookiesFile"];
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

