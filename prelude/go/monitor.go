//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbMonitorCheckOwner is Monitor#mon_check_owner: MRI's ThreadError unless the calling thread holds the monitor.
func rbMonitorCheckOwner(m *Monitor) {
	if m.m.owner.Load() != rbGoID() {
		panic(NewThreadError(Ref(String("current fiber not owner"))))
	}
}

// rbMonitorOnce is the Monitor in an object's @mon_data, made on first use: a
// compare-and-swap on that one field, so two threads agree on one monitor.
func rbMonitorOnce(slot **Monitor) *Monitor {
	p := (*unsafe.Pointer)(unsafe.Pointer(slot)) //nolint:gosec // atomic access to the object's own field
	if m := atomic.LoadPointer(p); m != nil {
		return (*Monitor)(m)
	}
	fresh := &Monitor{m: &Mutex{}}
	if atomic.CompareAndSwapPointer(p, nil, unsafe.Pointer(fresh)) { //nolint:gosec // as above
		return fresh
	}
	return (*Monitor)(atomic.LoadPointer(p))
}
