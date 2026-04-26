export namespace engine {
	
	export class Instance {
	    id: string;
	    name: string;
	    type: string;
	    version: string;
	    port: number;
	    dataDir: string;
	    logFile: string;
	    pidFile: string;
	    confFile?: string;
	    user: string;
	    status: string;
	    // Go type: time
	    createdAt: any;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new Instance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.type = source["type"];
	        this.version = source["version"];
	        this.port = source["port"];
	        this.dataDir = source["dataDir"];
	        this.logFile = source["logFile"];
	        this.pidFile = source["pidFile"];
	        this.confFile = source["confFile"];
	        this.user = source["user"];
	        this.status = source["status"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.lastError = source["lastError"];
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
	export class Version {
	    type: string;
	    major: string;
	    binPath: string;
	    installed: boolean;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new Version(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.major = source["major"];
	        this.binPath = source["binPath"];
	        this.installed = source["installed"];
	        this.label = source["label"];
	    }
	}

}

