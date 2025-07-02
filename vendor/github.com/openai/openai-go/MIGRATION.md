# OpenAI Go Migration Guide

<a href="https://pkg.go.dev/github.com/openai/openai-go"><img src="https://pkg.go.dev/badge/github.com/openai/openai-go.svg" alt="Go Reference"></a>

This SDK includes breaking changes to improve the ergonomics of constructing parameters and accessing responses.

To reduce verbosity, the `openai.F(...)` and `param.Field[T]` have been removed.
All calls to `openai.F(...)` can be deleted.

The SDK now uses the <code>\`json:"...,omitzero"\`</code> struct tag to omit fields. Nested structs, arrays and maps
can be declared like normal.

The old SDK used interfaces for unions in requests, which required
a type assertion to access variants and fields. The new design uses
structs with a field for each variant, wherein only one field can be set.
These struct unions also expose 'Get' methods to access and mutate subfields
which may be shared by multiple variants.

# Request parameters

## Required primitives parameters serialize their zero values (`string`, `int64`, etc.)

> [!CAUTION]
>
> **This change can cause new behavior in existing code, without compiler warnings.**

While migrating, ensure that all required fields are explicitly set. A required primitive
field `Age` will use the <code>\`json:"age,required"\`</code> struct tag without `omitzero`.

If a required primitive field is not set, the zero value will be serialized.
This was not the case in with `param.Field[T]`.

```diff
type FooParams struct {
-        Age  param.Field[int64]  `json:"age,required"`
-        Name param.Field[string] `json:"name"`
+        Age  int64               `json:"age,required"` // <== Notice no omitzero
+        Name param.Opt[string]   `json:"name,omitzero"`
}
```

<table>
<tr>
<th>Previous</th>
<th>New</th>
</tr>
<tr>
<td>

```go
_ = FooParams{
    Name: openai.String("Jerry")
}
`{"name": "Jerry"}` // (after serialization)
```

</td>
<td>

```go
_ = FooParams{
    Name: openai.String("Jerry")
}
`{"name": "Jerry", "age": 0}` // <== Notice the age field
```

</td>
</tr>
</table>

The required field `"age"` is now present as `0`. Fields without the <code>\`json:"...,omitzero"\`</code> struct tag
are always serialized, including their zero values.

## Transition from `param.Field[T]` to `omitzero`

The `openai.F(...)` function and `param.Field[T]` type are no longer present in the new SDK.

To represent omitted fields, the SDK uses <a href="https://pkg.go.dev/encoding/json#Marshal"><code>\`json:"...,omitzero"\`</code> semantics</a> from Go 1.24+ for JSON encoding[^1]. `omitzero` always omits fields
with zero values.

In all cases other than optional primitives, `openai.F()` can simply be removed.
For optional primitive types, such as `param.Opt[string]`, you can use `openai.String(string)` to construct the value.
Similar functions exist for other primitive types like `openai.Int(int)`, `openai.Bool(bool)`, etc.

`omitzero` is used for fields whose type is either a struct, slice, map, string enum,
or wrapped optional primitive (e.g. `param.Opt[T]`). Required primitive fields don't use `omitzero`.

**Example User Code: Constructing a request**

```diff
foo = FooParams{
-    RequiredString: openai.String("hello"),
+    RequiredString: "hello",

-    OptionalString: openai.String("hi"),
+    OptionalString: openai.String("hi"),

-    Array: openai.F([]BarParam{
-        BarParam{Prop: ... }
-    }),
+    Array: []BarParam{
+        BarParam{Prop: ... }
+    },

-    RequiredObject: openai.F(BarParam{ ... }),
+    RequiredObject: BarParam{ ... },

-    OptionalObject: openai.F(BarParam{ ... }),
+    OptionalObject: BarParam{ ... },

-    StringEnum: openai.F[BazEnum]("baz-ok"),
+    StringEnum: "baz-ok",
}
```

**Internal SDK Code: Fields of a request struct:**

```diff
type FooParams struct {
-    RequiredString param.Field[string]   `json:"required_string,required"`
+    RequiredString string                `json:"required_string,required"`

-    OptionalString param.Field[string]   `json:"optional_string"`
+    OptionalString param.Opt[string]     `json:"optional_string,omitzero"`

-    Array param.Field[[]BarParam]        `json"array"`
+    Array []BarParam                     `json"array,omitzero"`

-    Map param.Field[map[string]BarParam] `json"map"`
+    Map map[string]BarParam              `json"map,omitzero"`

-    RequiredObject param.Field[BarParam] `json:"required_object,required"`
+    RequiredObject BarParam              `json:"required_object,omitzero,required"`

-    OptionalObject param.Field[BarParam] `json:"optional_object"`
+    OptionalObject BarParam              `json:"optional_object,omitzero"`

-    StringEnum     param.Field[BazEnum]  `json:"string_enum"`
+    StringEnum     BazEnum               `json:"string_enum,omitzero"`
}
```

## Request Unions: Removing interfaces and moving to structs

For a type `AnimalUnionParam` which could be either a `CatParam | DogParam`.

<table>
<tr><th>Previous</th> <th>New</th></tr>
<tr>
<td>

```go
type AnimalParam interface {
	ImplAnimalParam()
}

func (Dog)         ImplAnimalParam() {}
func (Cat)         ImplAnimalParam() {}
```

</td>
<td>

```go
type AnimalUnionParam struct {
	OfCat 	 *Cat              `json:",omitzero,inline`
	OfDog    *Dog              `json:",omitzero,inline`
}
```

</td>
</tr>

<tr style="background:rgb(209, 217, 224)">
<td>

```go
var dog AnimalParam = DogParam{
	Name: "spot", ...
}
var cat AnimalParam = CatParam{
	Name: "whiskers", ...
}
```

</td>
<td>

```go
dog := AnimalUnionParam{
	OfDog: &DogParam{Name: "spot", ... },
}
cat := AnimalUnionParam{
	OfCat: &CatParam{Name: "whiskers", ... },
}
```

</td>
</tr>

<tr>
<td>

```go
var name string
switch v := animal.(type) {
case Dog:
	name = v.Name
case Cat:
	name = v.Name
}
```

</td>
<td>

```go
// Accessing fields
var name *string = animal.GetName()
```

</td>
</tr>
</table>

## Sending explicit `null` values

The old SDK had a function `param.Null[T]()` which could set `param.Field[T]` to `null`.

The new SDK uses `param.NullOpt[T]()` for to set a `param.Opt[T]` to `null`,
and `param.NullObj[T]()` to set a param struct `T` to `null`.

```diff
- var nullObj param.Field[BarParam] = param.Null[BarParam]()
+ var nullObj BarParam              = param.NullObj[BarParam]()

- var nullPrimitive param.Field[int64] = param.Null[int64]()
+ var nullPrimitive param.Opt[int64]   = param.NullOpt[int64]()
```

## Sending custom values

The `openai.Raw[T](any)` function has been removed. All request structs now support a
`.WithExtraField(map[string]any)` method to customize the fields.

```diff
foo := FooParams{
     A: param.String("hello"),
-    B: param.Raw[string](12) // sending `12` instead of a string
}
+ foo.WithExtraFields(map[string]any{
+    "B": 12,
+ })
```

# Response Properties

## Checking for presence of optional fields

The `.IsNull()` method has been changed to `.IsPresent()` to better reflect its behavior.

```diff
- if !resp.Foo.JSON.Bar.IsNull() {
+ if resp.Foo.JSON.Bar.IsPresent() {
    println("bar is present:", resp.Foo.Bar)
}
```

| Previous       | New                 | Returns true for values |
| -------------- | ------------------- | ----------------------- |
| `.IsNull()`    | `!.IsPresent()`     | `null` or Omitted       |
| `.IsMissing()` | `.Raw() == ""`      | Omitted                 |
|                | `.IsExplicitNull()` | `null`                  |

## Checking Raw JSON of a response

The `.RawJSON()` method has moved to the parent of the `.JSON` property.

```diff
- resp.Foo.JSON.RawJSON()
+ resp.Foo.RawJSON()
```

[^1]: The SDK doesn't require Go 1.24, despite supporting the `omitzero` feature
