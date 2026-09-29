//go:build ignore

// Package prelude is concatenated verbatim into the output (loadPreludeGo), never built for real: types like String come from generated code.
package prelude

// rbProcessStart anchors the monotonic clock; Go's time.Since reads the monotonic reading it carries.
var rbProcessStart = time.Now()

func rbClockGettime(id int) Float {
	switch id {
	case 0:
		return Float(float64(time.Now().UnixNano()) / 1e9)
	case 6:
		return Float(time.Since(rbProcessStart).Seconds())
	case 12:
		var ru syscall.Rusage
		if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
			panic(NewStandardError(Ref(String(err.Error()))))
		}
		return Float(time.Duration(ru.Utime.Nano() + ru.Stime.Nano()).Seconds())
	}
	panic(NewErrno_EINVAL(Ref(String("Invalid argument - clock_gettime"))))
}
