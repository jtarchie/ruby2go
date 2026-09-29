//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbObserverEntry is one Observable#add_observer registration (observer, and the Symbol method name notify_observers calls on it).
type rbObserverEntry struct {
	observer any
	fn       Symbol
}

// rbObservableState is Observable's per-object state, kept here (not in an `@ivar`) because a mixin module's methods compile generic over Self with no concrete Go field to write.
type rbObservableState struct {
	peers   []rbObserverEntry // insertion order, like a Hash
	changed bool
}

var (
	rbObservableMu     sync.Mutex
	rbObservableStates = map[any]*rbObservableState{}
)

// ponytail: entries are never removed, so every observable object leaks its state for the process lifetime; add a finalizer (runtime.AddCleanup) if that matters.
func rbObservableStateFor(self any) *rbObservableState {
	st := rbObservableStates[self]
	if st == nil {
		st = &rbObservableState{}
		rbObservableStates[self] = st
	}
	return st
}

func rbObservableAddObserver(self, observer any, fn Symbol) {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	st := rbObservableStateFor(self)
	for i, e := range st.peers {
		if e.observer == observer {
			st.peers[i].fn = fn
			return
		}
	}
	st.peers = append(st.peers, rbObserverEntry{observer: observer, fn: fn})
}

func rbObservableDeleteObserver(self, observer any) {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	st := rbObservableStates[self]
	if st == nil {
		return
	}
	for i, e := range st.peers {
		if e.observer == observer {
			st.peers = append(st.peers[:i], st.peers[i+1:]...)
			return
		}
	}
}

func rbObservableDeleteObservers(self any) {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	if st := rbObservableStates[self]; st != nil {
		st.peers = nil
	}
}

func rbObservableCountObservers(self any) int {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	if st := rbObservableStates[self]; st != nil {
		return len(st.peers)
	}
	return 0
}

func rbObservableSetChanged(self any, state bool) {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	rbObservableStateFor(self).changed = state
}

func rbObservableChanged(self any) bool {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	if st := rbObservableStates[self]; st != nil {
		return st.changed
	}
	return false
}

// rbObservableObservers and rbObservableFuncs snapshot the registered peers in parallel order; notify_observers zips them and calls `send` in ordinary Ruby, so dispatch goes through rb2go's normal computed-name `send` codegen, not reimplemented here.
func rbObservableObservers(self any) *Array[any] {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	st := rbObservableStates[self]
	if st == nil {
		out := Array[any]{}
		return &out
	}
	out := make(Array[any], len(st.peers))
	for i, e := range st.peers {
		out[i] = e.observer
	}
	return &out
}

func rbObservableFuncs(self any) *Array[Symbol] {
	rbObservableMu.Lock()
	defer rbObservableMu.Unlock()
	st := rbObservableStates[self]
	if st == nil {
		out := Array[Symbol]{}
		return &out
	}
	out := make(Array[Symbol], len(st.peers))
	for i, e := range st.peers {
		out[i] = e.fn
	}
	return &out
}
