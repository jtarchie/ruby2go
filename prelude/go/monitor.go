//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbMonitorCheckOwner is Monitor#mon_check_owner: MRI's ThreadError unless the calling thread holds the monitor.
func rbMonitorCheckOwner(m *Monitor) {
	if m.m.owner.Load() != rbGoID() {
		panic(NewThreadError(Ref(String("current fiber not owner"))))
	}
}

// rbMonitors are MonitorMixin's monitors, one per including object, by identity (decision 108).
// ponytail: entries are never removed, as Observable's (decision 74); add a finalizer if that matters.
var (
	rbMonitorsMu sync.Mutex
	rbMonitors   = map[any]*Monitor{}
)

func rbMonitorFor(self any) *Monitor {
	rbMonitorsMu.Lock()
	defer rbMonitorsMu.Unlock()
	m := rbMonitors[self]
	if m == nil {
		m = &Monitor{m: &Mutex{}}
		rbMonitors[self] = m
	}
	return m
}
