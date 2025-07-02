package sqlite3

import (
	"context"
	"io"
	"iter"
	"sync"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"

	"github.com/ncruces/go-sqlite3/internal/util"
)

// CollationNeeded registers a callback to be invoked
// whenever an unknown collation sequence is required.
//
// https://sqlite.org/c3ref/collation_needed.html
func (c *Conn) CollationNeeded(cb func(db *Conn, name string)) error {
	var enable int32
	if cb != nil {
		enable = 1
	}
	rc := res_t(c.call("sqlite3_collation_needed_go", stk_t(c.handle), stk_t(enable)))
	if err := c.error(rc); err != nil {
		return err
	}
	c.collation = cb
	return nil
}

// AnyCollationNeeded uses [Conn.CollationNeeded] to register
// a fake collating function for any unknown collating sequence.
// The fake collating function works like BINARY.
//
// This can be used to load schemas that contain
// one or more unknown collating sequences.
func (c Conn) AnyCollationNeeded() error {
	rc := res_t(c.call("sqlite3_anycollseq_init", stk_t(c.handle), 0, 0))
	if err := c.error(rc); err != nil {
		return err
	}
	c.collation = nil
	return nil
}

// CreateCollation defines a new collating sequence.
//
// https://sqlite.org/c3ref/create_collation.html
func (c *Conn) CreateCollation(name string, fn CollatingFunction) error {
	var funcPtr ptr_t
	defer c.arena.mark()()
	namePtr := c.arena.string(name)
	if fn != nil {
		funcPtr = util.AddHandle(c.ctx, fn)
	}
	rc := res_t(c.call("sqlite3_create_collation_go",
		stk_t(c.handle), stk_t(namePtr), stk_t(funcPtr)))
	return c.error(rc)
}

// Collating function is the type of a collation callback.
// Implementations must not retain a or b.
type CollatingFunction func(a, b []byte) int

// CreateFunction defines a new scalar SQL function.
//
// https://sqlite.org/c3ref/create_function.html
func (c *Conn) CreateFunction(name string, nArg int, flag FunctionFlag, fn ScalarFunction) error {
	var funcPtr ptr_t
	defer c.arena.mark()()
	namePtr := c.arena.string(name)
	if fn != nil {
		funcPtr = util.AddHandle(c.ctx, fn)
	}
	rc := res_t(c.call("sqlite3_create_function_go",
		stk_t(c.handle), stk_t(namePtr), stk_t(nArg),
		stk_t(flag), stk_t(funcPtr)))
	return c.error(rc)
}

// ScalarFunction is the type of a scalar SQL function.
// Implementations must not retain arg.
type ScalarFunction func(ctx Context, arg ...Value)

// CreateAggregateFunction defines a new aggregate SQL function.
//
// https://sqlite.org/c3ref/create_function.html
func (c *Conn) CreateAggregateFunction(name string, nArg int, flag FunctionFlag, fn AggregateSeqFunction) error {
	var funcPtr ptr_t
	defer c.arena.mark()()
	namePtr := c.arena.string(name)
	if fn != nil {
		funcPtr = util.AddHandle(c.ctx, AggregateConstructor(func() AggregateFunction {
			var a aggregateFunc
			coro := func(yieldCoro func(struct{}) bool) {
				seq := func(yieldSeq func([]Value) bool) {
					for yieldSeq(a.arg) {
						if !yieldCoro(struct{}{}) {
							break
						}
					}
				}
				fn(&a.ctx, seq)
			}
			a.next, a.stop = iter.Pull(coro)
			return &a
		}))
	}
	rc := res_t(c.call("sqlite3_create_aggregate_function_go",
		stk_t(c.handle), stk_t(namePtr), stk_t(nArg),
		stk_t(flag), stk_t(funcPtr)))
	return c.error(rc)
}

// AggregateSeqFunction is the type of an aggregate SQL function.
// Implementations must not retain the slices yielded by seq.
type AggregateSeqFunction func(ctx *Context, seq iter.Seq[[]Value])

// CreateWindowFunction defines a new aggregate or aggregate window SQL function.
// If fn returns a [WindowFunction], an aggregate window function is created.
// If fn returns an [io.Closer], it will be called to free resources.
//
// https://sqlite.org/c3ref/create_function.html
func (c *Conn) CreateWindowFunction(name string, nArg int, flag FunctionFlag, fn AggregateConstructor) error {
	var funcPtr ptr_t
	defer c.arena.mark()()
	namePtr := c.arena.string(name)
	if fn != nil {
		funcPtr = util.AddHandle(c.ctx, AggregateConstructor(func() AggregateFunction {
			agg := fn()
			if win, ok := agg.(WindowFunction); ok {
				return win
			}
			return windowFunc{agg, name}
		}))
	}
	rc := res_t(c.call("sqlite3_create_window_function_go",
		stk_t(c.handle), stk_t(namePtr), stk_t(nArg),
		stk_t(flag), stk_t(funcPtr)))
	return c.error(rc)
}

// AggregateConstructor is a an [AggregateFunction] constructor.
type AggregateConstructor func() AggregateFunction

// AggregateFunction is the interface an aggregate function should implement.
//
// https://sqlite.org/appfunc.html
type AggregateFunction interface {
	// Step is invoked to add a row to the current window.
	// The function arguments, if any, corresponding to the row being added, are passed to Step.
	// Implementations must not retain arg.
	Step(ctx Context, arg ...Value)

	// Value is invoked to return the current (or final) value of the aggregate.
	Value(ctx Context)
}

// WindowFunction is the interface an aggregate window function should implement.
//
// https://sqlite.org/windowfunctions.html
type WindowFunction interface {
	AggregateFunction

	// Inverse is invoked to remove the oldest presently aggregated result of Step from the current window.
	// The function arguments, if any, are those passed to Step for the row being removed.
	// Implementations must not retain arg.
	Inverse(ctx Context, arg ...Value)
}

// OverloadFunction overloads a function for a virtual table.
//
// https://sqlite.org/c3ref/overload_function.html
func (c *Conn) OverloadFunction(name string, nArg int) error {
	defer c.arena.mark()()
	namePtr := c.arena.string(name)
	rc := res_t(c.call("sqlite3_overload_function",
		stk_t(c.handle), stk_t(namePtr), stk_t(nArg)))
	return c.error(rc)
}

func destroyCallback(ctx context.Context, mod api.Module, pApp ptr_t) {
	util.DelHandle(ctx, pApp)
}

func collationCallback(ctx context.Context, mod api.Module, pArg, pDB ptr_t, eTextRep uint32, zName ptr_t) {
	if c, ok := ctx.Value(connKey{}).(*Conn); ok && c.handle == pDB && c.collation != nil {
		name := util.ReadString(mod, zName, _MAX_NAME)
		c.collation(c, name)
	}
}

func compareCallback(ctx context.Context, mod api.Module, pApp ptr_t, nKey1 int32, pKey1 ptr_t, nKey2 int32, pKey2 ptr_t) uint32 {
	fn := util.GetHandle(ctx, pApp).(CollatingFunction)
	return uint32(fn(util.View(mod, pKey1, int64(nKey1)), util.View(mod, pKey2, int64(nKey2))))
}

func funcCallback(ctx context.Context, mod api.Module, pCtx, pApp ptr_t, nArg int32, pArg ptr_t) {
	db := ctx.Value(connKey{}).(*Conn)
	args := callbackArgs(db, nArg, pArg)
	defer returnArgs(args)
	fn := util.GetHandle(db.ctx, pApp).(ScalarFunction)
	fn(Context{db, pCtx}, *args...)
}

func stepCallback(ctx context.Context, mod api.Module, pCtx, pAgg, pApp ptr_t, nArg int32, pArg ptr_t) {
	db := ctx.Value(connKey{}).(*Conn)
	args := callbackArgs(db, nArg, pArg)
	defer returnArgs(args)
	fn, _ := callbackAggregate(db, pAgg, pApp)
	fn.Step(Context{db, pCtx}, *args...)
}

func valueCallback(ctx context.Context, mod api.Module, pCtx, pAgg, pApp ptr_t, final int32) {
	db := ctx.Value(connKey{}).(*Conn)
	fn, handle := callbackAggregate(db, pAgg, pApp)
	fn.Value(Context{db, pCtx})

	// Cleanup.
	if final != 0 {
		var err error
		if handle != 0 {
			err = util.DelHandle(ctx, handle)
		} else if c, ok := fn.(io.Closer); ok {
			err = c.Close()
		}
		if err != nil {
			Context{db, pCtx}.ResultError(err)
			return // notest
		}
	}
}

func inverseCallback(ctx context.Context, mod api.Module, pCtx, pAgg ptr_t, nArg int32, pArg ptr_t) {
	db := ctx.Value(connKey{}).(*Conn)
	args := callbackArgs(db, nArg, pArg)
	defer returnArgs(args)
	fn := util.GetHandle(db.ctx, pAgg).(WindowFunction)
	fn.Inverse(Context{db, pCtx}, *args...)
}

func callbackAggregate(db *Conn, pAgg, pApp ptr_t) (AggregateFunction, ptr_t) {
	if pApp == 0 {
		handle := util.Read32[ptr_t](db.mod, pAgg)
		return util.GetHandle(db.ctx, handle).(AggregateFunction), handle
	}

	// We need to create the aggregate.
	fn := util.GetHandle(db.ctx, pApp).(AggregateConstructor)()
	if pAgg != 0 {
		handle := util.AddHandle(db.ctx, fn)
		util.Write32(db.mod, pAgg, handle)
		return fn, handle
	}
	return fn, 0
}

var (
	valueArgsPool sync.Pool
	valueArgsLen  atomic.Int32
)

func callbackArgs(db *Conn, nArg int32, pArg ptr_t) *[]Value {
	arg, ok := valueArgsPool.Get().(*[]Value)
	if !ok || cap(*arg) < int(nArg) {
		max := valueArgsLen.Or(nArg) | nArg
		lst := make([]Value, max)
		arg = &lst
	}
	lst := (*arg)[:nArg]
	for i := range lst {
		lst[i] = Value{
			c:      db,
			handle: util.Read32[ptr_t](db.mod, pArg+ptr_t(i)*ptrlen),
		}
	}
	*arg = lst
	return arg
}

func returnArgs(p *[]Value) {
	valueArgsPool.Put(p)
}

type aggregateFunc struct {
	next func() (struct{}, bool)
	stop func()
	ctx  Context
	arg  []Value
}

func (a *aggregateFunc) Step(ctx Context, arg ...Value) {
	a.ctx = ctx
	a.arg = append(a.arg[:0], arg...)
	if _, more := a.next(); !more {
		a.stop()
	}
}

func (a *aggregateFunc) Value(ctx Context) {
	a.ctx = ctx
	a.stop()
}

func (a *aggregateFunc) Close() error {
	a.stop()
	return nil
}

type windowFunc struct {
	AggregateFunction
	name string
}

func (w windowFunc) Inverse(ctx Context, arg ...Value) {
	// Implementing inverse allows certain queries that don't really need it to succeed.
	ctx.ResultError(util.ErrorString(w.name + ": may not be used as a window function"))
}
