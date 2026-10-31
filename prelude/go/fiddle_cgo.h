#define _GNU_SOURCE
#include <dlfcn.h>
#include <errno.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
	uintptr_t a[8];
	uint64_t f[8];
	uintptr_t s[16]; // the stack arguments, laid out by rbFiddlePlace
	uintptr_t r;
	uint64_t rf;
	int err;
} rb_fiddle_args;

typedef uintptr_t (*rb_fiddle_fn)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
	double, double, double, double, double, double, double, double);
typedef double (*rb_fiddle_fnf)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
	double, double, double, double, double, double, double, double);

#define RB_FIDDLE_STACK_TYPES uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, \
	uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t
#define RB_FIDDLE_STACK s->s[0], s->s[1], s->s[2], s->s[3], s->s[4], s->s[5], s->s[6], s->s[7], \
	s->s[8], s->s[9], s->s[10], s->s[11], s->s[12], s->s[13], s->s[14], s->s[15]

// decision 326's superset signature; errno is read in this frame, on the calling thread.
static void rb_fiddle_call(uintptr_t fn, int fret, rb_fiddle_args *s) {
	double f[8];
	memcpy(f, s->f, sizeof f);
	if (fret) {
		double d = ((rb_fiddle_fnf)fn)(s->a[0], s->a[1], s->a[2], s->a[3], s->a[4], s->a[5], s->a[6], s->a[7],
			f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7]);
		s->err = errno;
		memcpy(&s->rf, &d, sizeof d);
	} else {
		s->r = ((rb_fiddle_fn)fn)(s->a[0], s->a[1], s->a[2], s->a[3], s->a[4], s->a[5], s->a[6], s->a[7],
			f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7]);
		s->err = errno;
	}
}

// The superset plus 16 stack words; on x86-64 through a variadic prototype, so %al counts the vector registers a
// variadic callee saves (a fixed callee ignores it), and its 6 integer registers come before the stack words.
#if defined(__x86_64__)
typedef uintptr_t (*rb_fiddle_fns)(uintptr_t, ...);
typedef double (*rb_fiddle_fnsf)(uintptr_t, ...);
#define RB_FIDDLE_SPREAD_ARGS s->a[0], s->a[1], s->a[2], s->a[3], s->a[4], s->a[5], \
	f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7], RB_FIDDLE_STACK
#else
typedef uintptr_t (*rb_fiddle_fns)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
	double, double, double, double, double, double, double, double, RB_FIDDLE_STACK_TYPES);
typedef double (*rb_fiddle_fnsf)(uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t, uintptr_t,
	double, double, double, double, double, double, double, double, RB_FIDDLE_STACK_TYPES);
#define RB_FIDDLE_SPREAD_ARGS s->a[0], s->a[1], s->a[2], s->a[3], s->a[4], s->a[5], s->a[6], s->a[7], \
	f[0], f[1], f[2], f[3], f[4], f[5], f[6], f[7], RB_FIDDLE_STACK
#endif

static void rb_fiddle_call_spread(uintptr_t fn, int fret, rb_fiddle_args *s) {
	double f[8];
	memcpy(f, s->f, sizeof f);
	if (fret) {
		double d = ((rb_fiddle_fnsf)fn)(RB_FIDDLE_SPREAD_ARGS);
		s->err = errno;
		memcpy(&s->rf, &d, sizeof d);
	} else {
		s->r = ((rb_fiddle_fns)fn)(RB_FIDDLE_SPREAD_ARGS);
		s->err = errno;
	}
}

static uintptr_t rb_fiddle_default(void) { return (uintptr_t)RTLD_DEFAULT; }

static int rb_fiddle_dlclose(uintptr_t h) { return dlclose((void *)h); }

static uintptr_t rb_fiddle_dlsym(uintptr_t h, const char *name) { return (uintptr_t)dlsym((void *)h, name); }

// rugaroo:if rbFiddleCgoCallback
extern uintptr_t rbFiddleCgoCallback(int, uintptr_t *, uint64_t *);
extern double rbFiddleCgoCallbackF(int, uintptr_t *, uint64_t *);

#define RB_FIDDLE_CB_ARGS uintptr_t i0, uintptr_t i1, uintptr_t i2, uintptr_t i3, uintptr_t i4, uintptr_t i5, uintptr_t i6, \
	uintptr_t i7, double f0, double f1, double f2, double f3, double f4, double f5, double f6, double f7
#define RB_FIDDLE_CB_BODY uintptr_t in[8] = {i0, i1, i2, i3, i4, i5, i6, i7}; double fl[8] = {f0, f1, f2, f3, f4, f5, f6, f7};
#define RB_FIDDLE_CB(id, n) \
	static uintptr_t rb_fiddle_cb_##id(RB_FIDDLE_CB_ARGS) { RB_FIDDLE_CB_BODY return rbFiddleCgoCallback(n, in, (uint64_t *)fl); } \
	static double rb_fiddle_cbf_##id(RB_FIDDLE_CB_ARGS) { RB_FIDDLE_CB_BODY return rbFiddleCgoCallbackF(n, in, (uint64_t *)fl); }
#define RB_FIDDLE_CB10(id, n) RB_FIDDLE_CB(id##0, (n)*10) RB_FIDDLE_CB(id##1, (n)*10+1) RB_FIDDLE_CB(id##2, (n)*10+2) \
	RB_FIDDLE_CB(id##3, (n)*10+3) RB_FIDDLE_CB(id##4, (n)*10+4) RB_FIDDLE_CB(id##5, (n)*10+5) RB_FIDDLE_CB(id##6, (n)*10+6) \
	RB_FIDDLE_CB(id##7, (n)*10+7) RB_FIDDLE_CB(id##8, (n)*10+8) RB_FIDDLE_CB(id##9, (n)*10+9)
#define RB_FIDDLE_CB100(id, n) RB_FIDDLE_CB10(id##0, (n)*10) RB_FIDDLE_CB10(id##1, (n)*10+1) RB_FIDDLE_CB10(id##2, (n)*10+2) \
	RB_FIDDLE_CB10(id##3, (n)*10+3) RB_FIDDLE_CB10(id##4, (n)*10+4) RB_FIDDLE_CB10(id##5, (n)*10+5) \
	RB_FIDDLE_CB10(id##6, (n)*10+6) RB_FIDDLE_CB10(id##7, (n)*10+7) RB_FIDDLE_CB10(id##8, (n)*10+8) RB_FIDDLE_CB10(id##9, (n)*10+9)
#define RB_FIDDLE_CB1000(id, n) RB_FIDDLE_CB100(id##0, (n)*10) RB_FIDDLE_CB100(id##1, (n)*10+1) RB_FIDDLE_CB100(id##2, (n)*10+2) \
	RB_FIDDLE_CB100(id##3, (n)*10+3) RB_FIDDLE_CB100(id##4, (n)*10+4) RB_FIDDLE_CB100(id##5, (n)*10+5) \
	RB_FIDDLE_CB100(id##6, (n)*10+6) RB_FIDDLE_CB100(id##7, (n)*10+7) RB_FIDDLE_CB100(id##8, (n)*10+8) RB_FIDDLE_CB100(id##9, (n)*10+9)
RB_FIDDLE_CB1000(a, 0)
RB_FIDDLE_CB1000(b, 1)

#define RB_FIDDLE_AT(id) case (id): return f ? (uintptr_t)rb_fiddle_cbf_##id : (uintptr_t)rb_fiddle_cb_##id;
#define RB_FIDDLE_AT10(p, n) RB_FIDDLE_AT2(p##0, (n)*10) RB_FIDDLE_AT2(p##1, (n)*10+1) RB_FIDDLE_AT2(p##2, (n)*10+2) \
	RB_FIDDLE_AT2(p##3, (n)*10+3) RB_FIDDLE_AT2(p##4, (n)*10+4) RB_FIDDLE_AT2(p##5, (n)*10+5) RB_FIDDLE_AT2(p##6, (n)*10+6) \
	RB_FIDDLE_AT2(p##7, (n)*10+7) RB_FIDDLE_AT2(p##8, (n)*10+8) RB_FIDDLE_AT2(p##9, (n)*10+9)
#define RB_FIDDLE_AT2(id, n) case (n): return f ? (uintptr_t)rb_fiddle_cbf_##id : (uintptr_t)rb_fiddle_cb_##id;
#define RB_FIDDLE_AT100(p, n) RB_FIDDLE_AT10(p##0, (n)*10) RB_FIDDLE_AT10(p##1, (n)*10+1) RB_FIDDLE_AT10(p##2, (n)*10+2) \
	RB_FIDDLE_AT10(p##3, (n)*10+3) RB_FIDDLE_AT10(p##4, (n)*10+4) RB_FIDDLE_AT10(p##5, (n)*10+5) RB_FIDDLE_AT10(p##6, (n)*10+6) \
	RB_FIDDLE_AT10(p##7, (n)*10+7) RB_FIDDLE_AT10(p##8, (n)*10+8) RB_FIDDLE_AT10(p##9, (n)*10+9)
#define RB_FIDDLE_AT1000(p, n) RB_FIDDLE_AT100(p##0, (n)*10) RB_FIDDLE_AT100(p##1, (n)*10+1) RB_FIDDLE_AT100(p##2, (n)*10+2) \
	RB_FIDDLE_AT100(p##3, (n)*10+3) RB_FIDDLE_AT100(p##4, (n)*10+4) RB_FIDDLE_AT100(p##5, (n)*10+5) \
	RB_FIDDLE_AT100(p##6, (n)*10+6) RB_FIDDLE_AT100(p##7, (n)*10+7) RB_FIDDLE_AT100(p##8, (n)*10+8) RB_FIDDLE_AT100(p##9, (n)*10+9)

// rb_fiddle_cb_addr is callback n's C address, double-returning when f.
static uintptr_t rb_fiddle_cb_addr(int n, int f) {
	switch (n) {
	RB_FIDDLE_AT1000(a, 0)
	RB_FIDDLE_AT1000(b, 1)
	}
	return 0;
}
// rugaroo:end
