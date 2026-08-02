export namespace config {
	
	export class Group {
	    name: string;
	    macs: string[];
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.macs = source["macs"];
	    }
	}
	export class SavedDevice {
	    name: string;
	    ip: string;
	    port: string;
	    mac?: string;
	
	    static createFrom(source: any = {}) {
	        return new SavedDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.mac = source["mac"];
	    }
	}
	export class Config {
	    version: number;
	    ip: string;
	    port: string;
	    savedDevices?: SavedDevice[];
	    lastColor?: string;
	    lastBrightness?: number;
	    lastColorTemp?: number;
	    theme?: string;
	    lastScene?: number;
	    groups?: Group[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.savedDevices = this.convertValues(source["savedDevices"], SavedDevice);
	        this.lastColor = source["lastColor"];
	        this.lastBrightness = source["lastBrightness"];
	        this.lastColorTemp = source["lastColorTemp"];
	        this.theme = source["theme"];
	        this.lastScene = source["lastScene"];
	        this.groups = this.convertValues(source["groups"], Group);
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
	

}

export namespace main {
	
	export class FanoutResult {
	    ok: number;
	    failed: string[];
	    ms: number;
	
	    static createFrom(source: any = {}) {
	        return new FanoutResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.failed = source["failed"];
	        this.ms = source["ms"];
	    }
	}
	export class StateResult {
	    power: boolean;
	    brightness: number;
	    colorHex: string;
	    temp: number;
	    ms: number;
	    err: string;
	
	    static createFrom(source: any = {}) {
	        return new StateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.power = source["power"];
	        this.brightness = source["brightness"];
	        this.colorHex = source["colorHex"];
	        this.temp = source["temp"];
	        this.ms = source["ms"];
	        this.err = source["err"];
	    }
	}
	export class Target {
	    ip: string;
	    port: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.name = source["name"];
	    }
	}

}

export namespace wiz {
	
	export class Device {
	    IP: string;
	    Mac: string;
	    Name: string;
	    Model: string;
	    Firmware: string;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.IP = source["IP"];
	        this.Mac = source["Mac"];
	        this.Name = source["Name"];
	        this.Model = source["Model"];
	        this.Firmware = source["Firmware"];
	    }
	}

}

