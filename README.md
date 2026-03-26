# go-jmespath - A JMESPath implementation in Go

**NOTE: This is a fork of [go-jmespath](https://github.com/jmespath/go-jmespath).**

[![Build Status](https://img.shields.io/travis/jmespath/go-jmespath.svg)](https://travis-ci.org/jmespath/go-jmespath)



go-jmespath is a GO implementation of JMESPath,
which is a query language for JSON.  It will take a JSON
document and transform it into another JSON document
through a JMESPath expression.

Using go-jmespath is really easy.  There's a single function
you use, `jmespath.search`:


```go
> import "github.com/anatoly-tenenev/go-jmespath"
>
> var jsondata = []byte(`{"foo": {"bar": {"baz": [0, 1, 2, 3, 4]}}}`) // your data
> var data interface{}
> err := json.Unmarshal(jsondata, &data)
> result, err := jmespath.Search("foo.bar.baz[2]", data)
result = 2
```

In the example we gave the ``search`` function input data of
`{"foo": {"bar": {"baz": [0, 1, 2, 3, 4]}}}` as well as the JMESPath
expression `foo.bar.baz[2]`, and the `search` function evaluated
the expression against the input data to produce the result ``2``.

The JMESPath language can do a lot more than select an element
from a list.  Here are a few more examples:

```go
> var jsondata = []byte(`{"foo": {"bar": {"baz": [0, 1, 2, 3, 4]}}}`) // your data
> var data interface{}
> err := json.Unmarshal(jsondata, &data)
> result, err := jmespath.search("foo.bar", data)
result = { "baz": [ 0, 1, 2, 3, 4 ] }


> var jsondata  = []byte(`{"foo": [{"first": "a", "last": "b"},
                           {"first": "c", "last": "d"}]}`) // your data
> var data interface{}
> err := json.Unmarshal(jsondata, &data)
> result, err := jmespath.search({"foo[*].first", data)
result [ 'a', 'c' ]


> var jsondata = []byte(`{"foo": [{"age": 20}, {"age": 25},
                           {"age": 30}, {"age": 35},
                           {"age": 40}]}`) // your data
> var data interface{}
> err := json.Unmarshal(jsondata, &data)
> result, err := jmespath.search("foo[?age > `30`]")
result = [ { age: 35 }, { age: 40 } ]
```

You can also pre-compile your query. This is usefull if 
you are going to run multiple searches with it:

```go
	> var jsondata = []byte(`{"foo": "bar"}`)
	> var data interface{}
    > err := json.Unmarshal(jsondata, &data)
	> precompiled, err := Compile("foo")
	> if err != nil{
    >   // ... handle the error
    > }
    > result, err := precompiled.Search(data)
	result = "bar"
```

## Schema-aware Compile

For user-provided expressions you can validate paths and types at compile-time.
In schema-aware mode (`CompileWithSchema` / `CompileWithCompiledSchema`) this includes
function validation (unknown function names, arity, and argument types):

```go
schema := jmespath.JSONSchema{
	"type": "object",
	"properties": map[string]interface{}{
		"items": map[string]interface{}{
			"type": "array",
			"items": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"price": map[string]interface{}{"type": "number"},
				},
				"additionalProperties": false,
			},
		},
	},
	"additionalProperties": false,
}

// Fast path for many expressions against one schema:
cs, err := jmespath.CompileSchema(schema)
if err != nil {
	// handle unsupported schema
}

jp, err := jmespath.CompileWithCompiledSchema("items[].price", cs)
if err != nil {
	// err can be *jmespath.StaticError with Code/Offset
}

// Static type inference without runtime data:
inferred, err := jmespath.InferTypeWithCompiledSchema("items[].price", cs)
if err != nil {
	// err can be *jmespath.StaticError with Code/Offset
}
// inferred.Mask == jmespath.TypeArray
// inferred.Item.Mask == jmespath.TypeNumber

// Guard analysis on compiled expression:
guarded, err := jmespath.CompileWithCompiledSchema("items && items != `null`", cs)
if err != nil {
	// err can be *jmespath.StaticError with Code/Offset
}
if guarded.ProtectsWhenTrue("items") {
	// when expression is true, "items" is guaranteed non-missing/non-null
}
paths := guarded.GuardsWhenTrue().ProtectedPaths() // copy of guarded paths
_ = paths

// Guard analysis is enabled by default. You can disable it explicitly:
noGuards, err := jmespath.CompileWithCompiledSchemaOptions(
	"items && items != `null`",
	cs,
	&jmespath.SchemaCompileOptions{DisableGuardAnalysis: true},
)
if err != nil {
	// err can be *jmespath.StaticError with Code/Offset
}
_ = noGuards
```

`CompileWithSchema` is a convenience wrapper over `CompileSchema + CompileWithCompiledSchema`.

`InferTypeWithSchema` is a convenience wrapper over `CompileSchema + InferTypeWithCompiledSchema`.
For high-throughput usage, prefer `InferTypeWithCompiledSchema` to avoid recompiling schema.

`CompileSchema` validates the supported subset strictly: unknown schema keywords fail with `unsupported_schema`, while metadata fields `title`, `description`, `default`, and `examples` are ignored.

## More Resources

The example above only show a small amount of what
a JMESPath expression can do.  If you want to take a
tour of the language, the *best* place to go is the
[JMESPath Tutorial](http://jmespath.org/tutorial.html).

One of the best things about JMESPath is that it is
implemented in many different programming languages including
python, ruby, php, lua, etc.  To see a complete list of libraries,
check out the [JMESPath libraries page](http://jmespath.org/libraries.html).

And finally, the full JMESPath specification can be found
on the [JMESPath site](http://jmespath.org/specification.html).
