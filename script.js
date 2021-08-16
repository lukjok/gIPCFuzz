//console.log(JSON.stringify(Process.enumerateModules()));
console.log("Script start");
var gc_cnt = 0;
Stalker.trustThreshold = 0;
var stalkerEvents = [];
var handlerAddresses = [];

rpc.exports = {
	startCoverageFeed: function() {
		//TODO: multiple interceptors with stalker events
		Interceptor.attach(handlerAddresses[0].address, {
            onEnter: function (args) {
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
                        stalkerEvents.push({module: handlerAddresses[0].module, coverage: bbs});
                    }
                });
            },
            onLeave: function (retval) {
                Stalker.unfollow(Process.getCurrentThreadId());
                Stalker.flush();
				gc_cnt++;
				if (gc_cnt % 100 === 0) {
					Stalker.garbagecollect();
				}
            }
		});
	},
	setTargets: function(targets) {
		//[0] because thats how its being sent from GO side...
		if(targets[0] == null) {
			return false;
		}
		if(targets[0].length === 0) {
			return false;
		}
		targets[0].forEach((t) => {
			if(t.handler.startsWith("0x")) {
				handlerAddresses.push({module: t.module, address: new ptr(t.handler)});
			} else {
				var modAddr = Module.findExportByName(t.module, t.handler);
				if (modAddr != null) {
					handlerAddresses.push({module: t.module, address: modAddr});
				}
			}		
		});
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
	clearCoverage: function() {
		stalkerEvents = [];
	}
}