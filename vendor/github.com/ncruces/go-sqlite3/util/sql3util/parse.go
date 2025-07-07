package sql3util

import (
	"context"
	_ "embed"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/ncruces/go-sqlite3/internal/util"
)

const (
	errp = 4
	sqlp = 8
)

var (
	//go:embed wasm/sql3parse_table.wasm
	binary   []byte
	once     sync.Once
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
)

// ParseTable parses a [CREATE] or [ALTER TABLE] command.
//
// [CREATE]: https://sqlite.org/lang_createtable.html
// [ALTER TABLE]: https://sqlite.org/lang_altertable.html
func ParseTable(sql string) (_ *Table, err error) {
	once.Do(func() {
		ctx := context.Background()
		cfg := wazero.NewRuntimeConfigInterpreter()
		runtime = wazero.NewRuntimeWithConfig(ctx, cfg)
		compiled, err = runtime.CompileModule(ctx, binary)
	})
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithName(""))
	if err != nil {
		return nil, err
	}
	defer mod.Close(ctx)

	if buf, ok := mod.Memory().Read(sqlp, uint32(len(sql))); ok {
		copy(buf, sql)
	}

	stack := [...]util.Stk_t{sqlp, util.Stk_t(len(sql)), errp}
	err = mod.ExportedFunction("sql3parse_table").CallWithStack(ctx, stack[:])
	if err != nil {
		return nil, err
	}

	c, _ := mod.Memory().ReadUint32Le(errp)
	switch c {
	case _MEMORY:
		panic(util.OOMErr)
	case _SYNTAX:
		return nil, util.ErrorString("sql3parse: invalid syntax")
	case _UNSUPPORTEDSQL:
		return nil, util.ErrorString("sql3parse: unsupported SQL")
	}

	var tab Table
	tab.load(mod, uint32(stack[0]), sql)
	return &tab, nil
}

// Table holds metadata about a table.
type Table struct {
	Name           string
	Schema         string
	Comment        string
	IsTemporary    bool
	IsIfNotExists  bool
	IsWithoutRowID bool
	IsStrict       bool
	Columns        []Column
	Type           StatementType
	CurrentName    string
	NewName        string
}

func (t *Table) load(mod api.Module, ptr uint32, sql string) {
	t.Name = loadString(mod, ptr+0, sql)
	t.Schema = loadString(mod, ptr+8, sql)
	t.Comment = loadString(mod, ptr+16, sql)

	t.IsTemporary = loadBool(mod, ptr+24)
	t.IsIfNotExists = loadBool(mod, ptr+25)
	t.IsWithoutRowID = loadBool(mod, ptr+26)
	t.IsStrict = loadBool(mod, ptr+27)

	t.Columns = loadSlice(mod, ptr+28, func(ptr uint32, ret *Column) {
		p, _ := mod.Memory().ReadUint32Le(ptr)
		ret.load(mod, p, sql)
	})

	t.Type = loadEnum[StatementType](mod, ptr+44)
	t.CurrentName = loadString(mod, ptr+48, sql)
	t.NewName = loadString(mod, ptr+56, sql)
}

// Column holds metadata about a column.
type Column struct {
	Name                  string
	Type                  string
	Length                string
	ConstraintName        string
	Comment               string
	IsPrimaryKey          bool
	IsAutoIncrement       bool
	IsNotNull             bool
	IsUnique              bool
	PKOrder               OrderClause
	PKConflictClause      ConflictClause
	NotNullConflictClause ConflictClause
	UniqueConflictClause  ConflictClause
	CheckExpr             string
	DefaultExpr           string
	CollateName           string
	ForeignKeyClause      *ForeignKey
}

func (c *Column) load(mod api.Module, ptr uint32, sql string) {
	c.Name = loadString(mod, ptr+0, sql)
	c.Type = loadString(mod, ptr+8, sql)
	c.Length = loadString(mod, ptr+16, sql)
	c.ConstraintName = loadString(mod, ptr+24, sql)
	c.Comment = loadString(mod, ptr+32, sql)

	c.IsPrimaryKey = loadBool(mod, ptr+40)
	c.IsAutoIncrement = loadBool(mod, ptr+41)
	c.IsNotNull = loadBool(mod, ptr+42)
	c.IsUnique = loadBool(mod, ptr+43)

	c.PKOrder = loadEnum[OrderClause](mod, ptr+44)
	c.PKConflictClause = loadEnum[ConflictClause](mod, ptr+48)
	c.NotNullConflictClause = loadEnum[ConflictClause](mod, ptr+52)
	c.UniqueConflictClause = loadEnum[ConflictClause](mod, ptr+56)

	c.CheckExpr = loadString(mod, ptr+60, sql)
	c.DefaultExpr = loadString(mod, ptr+68, sql)
	c.CollateName = loadString(mod, ptr+76, sql)

	if ptr, _ := mod.Memory().ReadUint32Le(ptr + 84); ptr != 0 {
		c.ForeignKeyClause = &ForeignKey{}
		c.ForeignKeyClause.load(mod, ptr, sql)
	}
}

type ForeignKey struct {
	Table      string
	Columns    []string
	OnDelete   FKAction
	OnUpdate   FKAction
	Match      string
	Deferrable FKDefType
}

func (f *ForeignKey) load(mod api.Module, ptr uint32, sql string) {
	f.Table = loadString(mod, ptr+0, sql)

	f.Columns = loadSlice(mod, ptr+8, func(ptr uint32, ret *string) {
		*ret = loadString(mod, ptr, sql)
	})

	f.OnDelete = loadEnum[FKAction](mod, ptr+16)
	f.OnUpdate = loadEnum[FKAction](mod, ptr+20)
	f.Match = loadString(mod, ptr+24, sql)
	f.Deferrable = loadEnum[FKDefType](mod, ptr+32)
}

func loadString(mod api.Module, ptr uint32, sql string) string {
	off, _ := mod.Memory().ReadUint32Le(ptr + 0)
	if off == 0 {
		return ""
	}
	len, _ := mod.Memory().ReadUint32Le(ptr + 4)
	return sql[off-sqlp : off+len-sqlp]
}

func loadSlice[T any](mod api.Module, ptr uint32, fn func(uint32, *T)) []T {
	ref, _ := mod.Memory().ReadUint32Le(ptr + 4)
	if ref == 0 {
		return nil
	}
	len, _ := mod.Memory().ReadUint32Le(ptr + 0)
	ret := make([]T, len)
	for i := range ret {
		fn(ref, &ret[i])
		ref += 4
	}
	return ret
}

func loadEnum[T ~uint32](mod api.Module, ptr uint32) T {
	val, _ := mod.Memory().ReadUint32Le(ptr)
	return T(val)
}

func loadBool(mod api.Module, ptr uint32) bool {
	val, _ := mod.Memory().ReadByte(ptr)
	return val != 0
}
