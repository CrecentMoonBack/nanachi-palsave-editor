export namespace main {
	
	export class ItemChoice {
	    id: string;
	    name: string;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new ItemChoice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.icon = source["icon"];
	    }
	}
	export class ItemInfo {
	    slot: number;
	    itemId: string;
	    name: string;
	    count: number;
	    icon: string;
	
	    static createFrom(source: any = {}) {
	        return new ItemInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slot = source["slot"];
	        this.itemId = source["itemId"];
	        this.name = source["name"];
	        this.count = source["count"];
	        this.icon = source["icon"];
	    }
	}
	export class PalInfo {
	    instanceId: string;
	    speciesId: string;
	    name: string;
	    nickname: string;
	    isBoss: boolean;
	    icon: string;
	    level: number;
	    exp: number;
	    rank: number;
	    talentHp: number;
	    talentMelee: number;
	    talentShot: number;
	    talentDefense: number;
	    passives: string[];
	
	    static createFrom(source: any = {}) {
	        return new PalInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.speciesId = source["speciesId"];
	        this.name = source["name"];
	        this.nickname = source["nickname"];
	        this.isBoss = source["isBoss"];
	        this.icon = source["icon"];
	        this.level = source["level"];
	        this.exp = source["exp"];
	        this.rank = source["rank"];
	        this.talentHp = source["talentHp"];
	        this.talentMelee = source["talentMelee"];
	        this.talentShot = source["talentShot"];
	        this.talentDefense = source["talentDefense"];
	        this.passives = source["passives"];
	    }
	}
	export class PlayerInfo {
	    uid: string;
	    name: string;
	    level: number;
	    palCount: number;
	    hasSave: boolean;
	    itemCount: number;
	
	    static createFrom(source: any = {}) {
	        return new PlayerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.name = source["name"];
	        this.level = source["level"];
	        this.palCount = source["palCount"];
	        this.hasSave = source["hasSave"];
	        this.itemCount = source["itemCount"];
	    }
	}
	export class SaveInfo {
	    path: string;
	    format: string;
	    engine: string;
	    sizeBytes: number;
	    playerCount: number;
	    palCount: number;
	    itemSlots: number;
	    playerSaves: number;
	
	    static createFrom(source: any = {}) {
	        return new SaveInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.format = source["format"];
	        this.engine = source["engine"];
	        this.sizeBytes = source["sizeBytes"];
	        this.playerCount = source["playerCount"];
	        this.palCount = source["palCount"];
	        this.itemSlots = source["itemSlots"];
	        this.playerSaves = source["playerSaves"];
	    }
	}
	export class SaveResult {
	    backupPath: string;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backupPath = source["backupPath"];
	        this.sizeBytes = source["sizeBytes"];
	    }
	}
	export class SpeciesSummary {
	    speciesId: string;
	    name: string;
	    icon: string;
	    count: number;
	    minLevel: number;
	    maxLevel: number;
	
	    static createFrom(source: any = {}) {
	        return new SpeciesSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.speciesId = source["speciesId"];
	        this.name = source["name"];
	        this.icon = source["icon"];
	        this.count = source["count"];
	        this.minLevel = source["minLevel"];
	        this.maxLevel = source["maxLevel"];
	    }
	}
	export class Status {
	    codecOk: boolean;
	    codecError: string;
	    iconsOk: boolean;
	    iconCount: number;
	    saveOpen: boolean;
	    savePath: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.codecOk = source["codecOk"];
	        this.codecError = source["codecError"];
	        this.iconsOk = source["iconsOk"];
	        this.iconCount = source["iconCount"];
	        this.saveOpen = source["saveOpen"];
	        this.savePath = source["savePath"];
	    }
	}

}

