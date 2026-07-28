export namespace bridge {
	
	export class GetHistoryFilterMatchesForEntriesParams {
	    entryIds: number[];
	
	    static createFrom(source: any = {}) {
	        return new GetHistoryFilterMatchesForEntriesParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entryIds = source["entryIds"];
	    }
	}
	export class HTTPResponseDetail {
	    id: number;
	    host: string;
	    port: string;
	    headBlockStr: string;
	    bodyStr: string;
	    truncatedBody: boolean;
	    version: string;
	    statusCode: number;
	    unsupportedContentEncodings: string[];
	    contentDecodingFailed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HTTPResponseDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.headBlockStr = source["headBlockStr"];
	        this.bodyStr = source["bodyStr"];
	        this.truncatedBody = source["truncatedBody"];
	        this.version = source["version"];
	        this.statusCode = source["statusCode"];
	        this.unsupportedContentEncodings = source["unsupportedContentEncodings"];
	        this.contentDecodingFailed = source["contentDecodingFailed"];
	    }
	}
	export class HTTPRequestDetail {
	    id: number;
	    host: string;
	    port: string;
	    headBlockStr: string;
	    bodyStr: string;
	    truncatedBody: boolean;
	    version: string;
	    method: string;
	    path: string;
	    scheme: string;
	
	    static createFrom(source: any = {}) {
	        return new HTTPRequestDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.headBlockStr = source["headBlockStr"];
	        this.bodyStr = source["bodyStr"];
	        this.truncatedBody = source["truncatedBody"];
	        this.version = source["version"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.scheme = source["scheme"];
	    }
	}
	export class HTTPHistoryEntryDetail {
	    id: number;
	    request?: HTTPRequestDetail;
	    response?: HTTPResponseDetail;
	
	    static createFrom(source: any = {}) {
	        return new HTTPHistoryEntryDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.request = this.convertValues(source["request"], HTTPRequestDetail);
	        this.response = this.convertValues(source["response"], HTTPResponseDetail);
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
	
	
	export class HistoryFilterMatch {
	    filterId: string;
	    version: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryFilterMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filterId = source["filterId"];
	        this.version = source["version"];
	    }
	}
	export class HistoryEntryFilterMatches {
	    entryId: number;
	    matches: HistoryFilterMatch[];
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntryFilterMatches(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entryId = source["entryId"];
	        this.matches = this.convertValues(source["matches"], HistoryFilterMatch);
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
	export class HistoryFilterCondition {
	    query: string;
	    target: string;
	    matchMode: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryFilterCondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.target = source["target"];
	        this.matchMode = source["matchMode"];
	    }
	}
	
	export class RemoveActiveHistoryFilterParams {
	    filterId: string;
	    version: number;
	
	    static createFrom(source: any = {}) {
	        return new RemoveActiveHistoryFilterParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filterId = source["filterId"];
	        this.version = source["version"];
	    }
	}
	export class UpsertActiveHistoryFilterParams {
	    filterId: string;
	    version: number;
	    conditions: HistoryFilterCondition[];
	    operator: string;
	
	    static createFrom(source: any = {}) {
	        return new UpsertActiveHistoryFilterParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filterId = source["filterId"];
	        this.version = source["version"];
	        this.conditions = this.convertValues(source["conditions"], HistoryFilterCondition);
	        this.operator = source["operator"];
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
	export class UpsertActiveHistoryFilterResult {
	    filterId: string;
	    version: number;
	    matchingIds: number[];
	
	    static createFrom(source: any = {}) {
	        return new UpsertActiveHistoryFilterResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filterId = source["filterId"];
	        this.version = source["version"];
	        this.matchingIds = source["matchingIds"];
	    }
	}

}

export namespace proxy {
	
	export class UpstreamProxyConfig {
	    enabled: boolean;
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new UpstreamProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}

}

export namespace repeater {
	
	export class RepeaterHttp2PseudoHeaders {
	    method: string;
	    scheme: string;
	    authority: string;
	    path: string;
	    protocol: string;
	
	    static createFrom(source: any = {}) {
	        return new RepeaterHttp2PseudoHeaders(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.scheme = source["scheme"];
	        this.authority = source["authority"];
	        this.path = source["path"];
	        this.protocol = source["protocol"];
	    }
	}
	export class RepeaterSendPayload {
	    scheme: string;
	    host: string;
	    port: string;
	    method: string;
	    path: string;
	    headBlockStr: string;
	    bodyStr: string;
	    skipServerCertVerify: boolean;
	    version: string;
	    pseudoHeaders: RepeaterHttp2PseudoHeaders;
	    headers: Record<string, Array<string>>;
	
	    static createFrom(source: any = {}) {
	        return new RepeaterSendPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scheme = source["scheme"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.headBlockStr = source["headBlockStr"];
	        this.bodyStr = source["bodyStr"];
	        this.skipServerCertVerify = source["skipServerCertVerify"];
	        this.version = source["version"];
	        this.pseudoHeaders = this.convertValues(source["pseudoHeaders"], RepeaterHttp2PseudoHeaders);
	        this.headers = source["headers"];
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
	export class RepeaterSendResult {
	    headBlockStr: string;
	    bodyStr: string;
	    host?: string;
	    port?: string;
	    version?: string;
	    statusCode?: number;
	    durationMs?: number;
	    unsupportedContentEncodings: string[];
	    contentDecodingFailed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RepeaterSendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.headBlockStr = source["headBlockStr"];
	        this.bodyStr = source["bodyStr"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.version = source["version"];
	        this.statusCode = source["statusCode"];
	        this.durationMs = source["durationMs"];
	        this.unsupportedContentEncodings = source["unsupportedContentEncodings"];
	        this.contentDecodingFailed = source["contentDecodingFailed"];
	    }
	}

}

export namespace settings {
	
	export class ProxyConfig {
	    schemaVersion: number;
	    proxyMode: string;
	    specificIp: string;
	    port: number;
	    skipServerCertVerify: boolean;
	    upstreamProxy: proxy.UpstreamProxyConfig;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.proxyMode = source["proxyMode"];
	        this.specificIp = source["specificIp"];
	        this.port = source["port"];
	        this.skipServerCertVerify = source["skipServerCertVerify"];
	        this.upstreamProxy = this.convertValues(source["upstreamProxy"], proxy.UpstreamProxyConfig);
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
	export class InitialAppState {
	    config: ProxyConfig;
	    configDirectory: string;
	    configFilePath: string;
	    caCertificatePath: string;
	    loadWarning: string;
	
	    static createFrom(source: any = {}) {
	        return new InitialAppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], ProxyConfig);
	        this.configDirectory = source["configDirectory"];
	        this.configFilePath = source["configFilePath"];
	        this.caCertificatePath = source["caCertificatePath"];
	        this.loadWarning = source["loadWarning"];
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

