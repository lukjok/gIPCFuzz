//console.log(JSON.stringify(Process.enumerateModules()));
//console.log("[*] Script start");
var gc_cnt = 0;
Stalker.trustThreshold = 0;
var stalkerEvents = [];
var covTarget = {};
var execTime = undefined;

rpc.exports = {
	startCoverageFeed: function() {
		//TODO: multiple interceptors with stalker events
		const module = Process.findModuleByName(covTarget.module);
		if (module == null) {
			throw "Cannot find specified module!";
		}

		//console.log("[*] Module: ", module.base)
		const funAddr = module.base.add(covTarget.address);
		//console.log("[*] Module function addr: ", funAddr)

		Interceptor.attach(funAddr, {
            onEnter: function (args) {
				execTime = Date.now()
                Stalker.follow(Process.getCurrentThreadId(), {
                    events: {
                        call: false,
                        ret: false,
                        exec: false,
                        block: false,
                        compile: true
                    },
                    onReceive: function (events) {
						const bbs = Stalker.parse(events, {
							stringify: true,
							annotate: false
						});
                        stalkerEvents.push({module: covTarget.module, coverage: bbs});
                    }
                });
            },
            onLeave: function (retval) {
				let tNow = Date.now();
				//console.log("[*] Enter time: ", execTime);
				execTime = tNow - execTime;
				//console.log("[*] Exit time: ", tNow);
				//console.log("[*] Execution duration: ", execTime);
                Stalker.unfollow(Process.getCurrentThreadId());
                Stalker.flush();
				gc_cnt++;
				if (gc_cnt % 100 === 0) {
					Stalker.garbagecollect();
				}
            }
		});
	},
	setTarget: function(target) {
		//[0] because thats how its being sent from GO side...
		// if(target[0] == null) {
		// 	return false;
		// }
		console.log("[*] Recieved target: ", JSON.stringify(target[0]));
		if(target[0].handler.startsWith("0x")) {
			covTarget = {module: target[0].module, address: new ptr(target[0].handler)};
		} else {
			var modAddr = Module.findExportByName(target[0].module, target[0].handler);
			if (modAddr != null) {				
				covTarget = {module: target[0].module, address: modAddr};
			}
		}		
		return true;
	},
	stopCoverageFeed: function() {
		Stalker.unfollow(Process.getCurrentThreadId());
		Stalker.flush();
		Stalker.garbageCollect()
		Interceptor.detachAll();
	},
	getCoverage: function() {
		return stalkerEvents;
	},
	getExecTime: function() {
		let tmpTime = execTime;
		execTime = undefined
		return tmpTime;
	},
	clearCoverage: function() {
		stalkerEvents = [];
	}
}

const Golang = {
	gopclntab: null,
	symbolmap: null,

	findGopclntab: function() {
		if (this.gopclntab) {
			return this.gopclntab;
		}
		const pattern = 'FB FF FF FF 00 00';
		var mainmodule = Process.enumerateModules()[0];
		var ranges = Process.enumerateRangesSync('r--');
		for (var ri in ranges) {
			var range = ranges[ri];
			if (range['base'] && range['size']) {
				var matches = Memory.scanSync(range['base'], range['size'], pattern);
				if (matches && matches.length > 0) {
					console.log(JSON.stringify(matches));
					for (var mi in matches) {
						// Check if this is the table by comparing its first entry defined
						// address with the defined offset + the found address.
						var address = matches[mi]['address'];
						var entry = address.add(8).add(Process.pointerSize).readPointer();
						var offset = address.add(8).add(Process.pointerSize * 2).readPointer();
						if (address.add(offset).readPointer().compare(entry) == 0) {
							console.log(address);
							this.gopclntab = address;
							return this.gopclntab;
						}
					};
				}
			}
		}
		return null;
	},

	enumerateSymbolsSync: function() {
		if (this.symbolmap) {
			return this.symbolmap;
		}
		var address = this.findGopclntab();
		var output = [];
		if(address) {
			// Read the header
			var headerSize = 8;
			var tableBase = address;
			var recordSize = Process.pointerSize * 2;
			var cursor = address.add(headerSize);
			var tableEnd = address.add(cursor.readUInt() * recordSize);
			cursor = cursor.add(Process.pointerSize);
			// Enumerate the records
			while ((cursor.compare(tableEnd) == -1)) {
				var offset = cursor.add(Process.pointerSize).readPointer();
				var functionAddress = tableBase.add(offset).readPointer();
				var nameOffset = tableBase.add(offset).add(Process.pointerSize).readU32();
				var name = tableBase.add(nameOffset).readUtf8String();
				output.push({
					address: functionAddress,
					name: name,
					table: tableBase.add(offset),
					tableBase: tableBase,
				})
				cursor = cursor.add(recordSize);
			}

		}
		this.symbolmap = output;
		return this.symbolmap;
	},

	findSymbolByName: function(name) {
		var map = this.enumerateSymbolsSync();
		for (var index in map) {
			if (map[index]['name'] === name) {
				return map[index]['address'];
			}
		}
		return null;
	},

	findSymbolsByPattern: function(pattern) {
		var map = this.enumerateSymbolsSync();
		var output = []
		for (var index in map) {
			if (map[index]['name'].match(pattern)) {
				output.push(map[index]);
			}
		}
		return output;
	},
};