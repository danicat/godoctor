Source: https://go.dev/wiki/CodeReviewComments\n\n# Go Wiki: Go Code Review Comments\n\n\n# Go Wiki: Go Code Review Comments\n\nThis page collects common comments made during reviews of Go code, so
that a single detailed explanation can be referred to by shorthands.
This is a laundry list of common style issues, not a comprehensive style guide.\n\nYou can view this as a supplement to Effective Go.\n\nAdditional comments related to testing can be found at Go Test Comments\n\nGoogle has published a longer Go Style Guide.\n\nPlease discuss changes before editing this page, even minor ones. Many people have opinions and this is not the place for edit wars.\n\n- Gofmt\n- Comment Sentences\n- Contexts\n- Copying\n- Crypto Rand\n- Declaring Empty Slices\n- Doc Comments\n- Don’t Panic\n- Error Strings\n- Examples\n- Goroutine Lifetimes\n- Handle Errors\n- Imports\n- Import Blank\n- Import Dot\n- In-Band Errors\n- Indent Error Flow\n- Initialisms\n- Interfaces\n- Line Length\n- Mixed Caps\n- Named Result Parameters\n- Naked Returns\n- Package Comments\n- Package Names\n- Pass Values\n- Receiver Names\n- Receiver Type\n- Synchronous Functions\n- Useful Test Failures\n- Variable Names\n\n## Gofmt\n\nRun gofmt on your code to automatically fix the majority of mechanical style issues. Almost all Go code in the wild uses gofmt. The rest of this document addresses non-mechanical style points.\n\nAn alternative is to use goimports, a superset of gofmt which additionally adds (and removes) import lines as necessary.\n\n## Comment Sentences\n\nSee https://go.dev/doc/effective_go#commentary. Comments documenting declarations should be full sentences, even if that seems a little redundant. This approach makes them format well when extracted into godoc documentation. Comments should begin with the name of the thing being described and end in a period:\n\n```\n// Request represents a request to run a command.
type Request struct { ...

// Encode writes the JSON encoding of req to w.
func Encode(w io.Writer, req *Request) { ...
\n```\n\nand so on.\n\n## Contexts\n\nValues of the context.Context type carry security credentials,
tracing information, deadlines, and cancellation signals across API
and process boundaries. Go programs pass Contexts explicitly along
the entire function call chain from incoming RPCs and HTTP requests
to outgoing requests.\n\nMost functions that use a Context should accept it as their first parameter:\n\n```\nfunc F(ctx context.Context, /* other arguments */) {}
\n```\n\nA function that is never request-specific may use context.Background(),
but err on the side of passing a Context even if you think you don’t need
to. The default case is to pass a Context; only use context.Background()
directly if you have a good reason why the alternative is a mistake.\n\nDon’t add a Context member to a struct type; instead add a ctx parameter
to each method on that type that needs to pass it along. The one exception
is for methods whose signature must match an interface in the standard library
or in a third party library.\n\nDon’t create custom Context types or use interfaces other than Context in
function signatures.\n\nIf you have application data to pass around, put it in a parameter,
in the receiver, in globals, or, if it truly belongs there, in a Context value.\n\nContexts are immutable, so it’s fine to pass the same ctx to multiple
calls that share the same deadline, cancellation signal, credentials,
parent trace, etc.\n\n## Copying\n\nTo avoid unexpected aliasing, be careful when copying a struct from another package.
For example, the bytes.Buffer type contains a []byte slice. If you copy a Buffer,
the slice in the copy may alias the array in the original, causing subsequent method
calls to have surprising effects.\n\nIn general, do not copy a value of type T if its methods are associated with the
pointer type, *T.\n\n## Crypto Rand\n\nDo not use package math/rand
or math/rand/v2 to generate keys, even throwaway ones.
Seeded with Time.Nanoseconds(),
there are just a few bits of entropy.
Instead, use crypto/rand.Reader.
If you need text, use crypto/rand.Text,
or alternatively, encode random bytes with encoding/hex
or encoding/base64.\n\n```\nimport (
    "crypto/rand"
    "fmt"
)

func Key() string {
  return rand.Text()
}
\n```\n\n## Declaring Empty Slices\n\nWhen declaring an empty slice, prefer\n\n```\nvar t []string
\n```\n\nover\n\n```\nt := []string{}
\n```\n\nThe former declares a nil slice value, while the latter is non-nil but zero-length. They are functionally equivalent—their len and cap are both zero—but the nil slice is the preferred style.\n\nNote that there are limited circumstances where a non-nil but zero-length slice is preferred, such as when encoding JSON objects (a nil slice encodes to null, while []string{} encodes to the JSON array []).\n\nWhen designing interfaces, avoid making a distinction between a nil slice and a non-nil, zero-length slice, as this can lead to subtle programming errors.\n\nFor more discussion about nil in Go see Francesc Campoy’s talk Understanding Nil.\n\n## Doc Comments\n\nAll top-level, exported names should have doc comments, as should non-trivial unexported type or function declarations. See https://go.dev/doc/effective_go#commentary for more information about commentary conventions.\n\n## Don’t Panic\n\nSee https://go.dev/doc/effective_go#errors. Don’t use panic for normal error handling. Use error and multiple return values.\n\n## Error Strings\n\nError strings should not be capitalized (unless beginning with proper nouns or acronyms) or end with punctuation, since they are usually printed following other context. That is, use fmt.Errorf("something bad") not fmt.Errorf("Something bad"), so that log.Printf("Reading %s: %v", filename, err) formats without a spurious capital letter mid-message. This does not apply to logging, which is implicitly line-oriented and not combined inside other messages.\n\n## Examples\n\nWhen adding a new package, include examples of intended usage: a runnable Example,
or a simple test demonstrating a complete call sequence.\n\nRead more about testable Example() functions.\n\n## Goroutine Lifetimes\n\nWhen you spawn goroutines, make it clear when - or whether - they exit.\n\nGoroutines can leak by blocking on channel sends or receives: the garbage collector
will not terminate a goroutine even if the channels it is blocked on are unreachable.\n\nEven when goroutines do not leak, leaving them in-flight when they are no longer
needed can cause other subtle and hard-to-diagnose problems. Sends on closed channels
panic. Modifying still-in-use inputs “after the result isn’t needed” can still lead
to data races. And leaving goroutines in-flight for arbitrarily long can lead to
unpredictable memory usage.\n\nTry to keep concurrent code simple enough that goroutine lifetimes are obvious.
If that just isn’t feasible, document when and why the goroutines exit.\n\n## Handle Errors\n\nSee https://go.dev/doc/effective_go#errors. Do not discard errors using _ variables. If a function returns an error, check it to make sure the function succeeded. Handle the error, return it, or, in truly exceptional situations, panic.\n\n## Imports\n\nAvoid renaming imports except to avoid a name collision; good package names
should not require renaming. In the event of collision, prefer to rename the most
local or project-specific import.\n\nImports are organized in groups, with blank lines between them.
The standard library packages are always in the first group.\n\n```\npackage main

import (
    "fmt"
    "hash/adler32"
    "os"

    "github.com/foo/bar"
    "rsc.io/goversion/version"
)
\n```\n\ngoimports will do this for you.\n\n## Import Blank\n\nPackages that are imported only for their side effects (using the syntax import _ "pkg") should only be imported in the main package of a program, or in tests
that require them.\n\n## Import Dot\n\nThe import . form can be useful in tests that, due to circular dependencies, cannot be made part of the package being tested:\n\n```\npackage foo_test

import (
    "bar/testutil" // also imports "foo"
    . "foo"
)
\n```\n\nIn this case, the test file cannot be in package foo because it uses bar/testutil, which imports foo. So we use the ‘import .’ form to let the file pretend to be part of package foo even though it is not. Except for this one case, do not use import . in your programs. It makes the programs much harder to read because it is unclear whether a name like Quux is a top-level identifier in the current package or in an imported package.\n\n## In-Band Errors\n\nIn C and similar languages, it’s common for functions to return values like -1
or null to signal errors or missing results:\n\n```\n// Lookup returns the value for key or "" if there is no mapping for key.
func Lookup(key string) string

// Failing to check for an in-band error value can lead to bugs:
Parse(Lookup(key))  // returns "parse failure for value" instead of "no value for key"
\n```\n\nGo’s support for multiple return values provides a better solution.
Instead of requiring clients to check for an in-band error value, a function should return
an additional value to indicate whether its other return values are valid. This return
value may be an error, or a boolean when no explanation is needed.
It should be the final return value.\n\n```\n// Lookup returns the value for key or ok=false if there is no mapping for key.
func Lookup(key string) (value string, ok bool)
\n```\n\nThis prevents the caller from using the result incorrectly:\n\n```\nParse(Lookup(key))  // compile-time error
\n```\n\nAnd encourages more robust and readable code:\n\n```\nvalue, ok := Lookup(key)
if !ok {
    return fmt.Errorf("no value for %q", key)
}
return Parse(value)
\n```\n\nThis rule applies to exported functions but is also useful
for unexported functions.\n\nReturn values like nil, “”, 0, and -1 are fine when they are
valid results for a function, that is, when the caller need not
handle them differently from other values.\n\nSome standard library functions, like those in package “strings”,
return in-band error values. This greatly simplifies string-manipulation
code at the cost of requiring more diligence from the programmer.
In general, Go code should return additional values for errors.\n\n## Indent Error Flow\n\nTry to keep the normal code path at a minimal indentation, and indent the error handling, dealing with it first. This improves the readability of the code by permitting visually scanning the normal path quickly. For instance, don’t write:\n\n```\nif err != nil {
    // error handling
} else {
    // normal code
}
\n```\n\nInstead, write:\n\n```\nif err != nil {
    // error handling
    return // or continue, etc.
}
// normal code
\n```\n\nIf the if statement has an initialization statement, such as:\n\n```\nif x, err := f(); err != nil {
    // error handling
    return
} else {
    // use x
}
\n```\n\nthen this may require moving the short variable declaration to its own line:\n\n```\nx, err := f()
if err != nil {
    // error handling
    return
}
// use x
\n```\n\n## Initialisms\n\nWords in names that are initialisms or acronyms (e.g. “URL” or “NATO”) have a consistent case. For example, “URL” should appear as “URL” or “url” (as in “urlPony”, or “URLPony”), never as “Url”. As an example: ServeHTTP not ServeHttp. For identifiers with multiple initialized “words”, use for example “xmlHTTPRequest” or “XMLHTTPRequest”.\n\nThis rule also applies to “ID” when it is short for “identifier” (which is pretty much all cases when it’s not the “id” as in “ego”, “superego”), so write “appID” instead of “appId”.\n\nCode generated by the protocol buffer compiler is exempt from this rule. Human-written code is held to a higher standard than machine-written code.\n\n## Interfaces\n\nGo interfaces generally belong in the package that uses values of the
interface type, not the package that implements those values. The
implementing package should return concrete (usually pointer or struct)
types: that way, new methods can be added to implementations without
requiring extensive refactoring.\n\nDo not define interfaces on the implementor side of an API “for mocking”;
instead, design the API so that it can be tested using the public API of
the real implementation.\n\nDo not define interfaces before they are used: without a realistic example
of usage, it is too difficult to see whether an interface is even necessary,
let alone what methods it ought to contain.\n\n```\npackage consumer  // consumer.go

type Thinger interface { Thing() bool }

func Foo(t Thinger) string { … }
\n```\n\n```\npackage consumer // consumer_test.go

type fakeThinger struct{ … }
func (t fakeThinger) Thing() bool { … }
…
if Foo(fakeThinger{…}) == "x" { … }
\n```\n\n```\n// DO NOT DO IT!!!
package producer

type Thinger interface { Thing() bool }

type defaultThinger struct{ … }
func (t defaultThinger) Thing() bool { … }

func NewThinger() Thinger { return defaultThinger{ … } }
\n```\n\nInstead return a concrete type and let the consumer mock the producer implementation.\n\n```\npackage producer

type Thinger struct{ … }
func (t Thinger) Thing() bool { … }

func NewThinger() Thinger { return Thinger{ … } }
\n```\n\n## Line Length\n\nThere is no rigid line length limit in Go code, but avoid uncomfortably long lines.
Similarly, don’t add line breaks to keep lines short when they are more readable long–for example,
if they are repetitive.\n\nMost of the time when people wrap lines “unnaturally” (in the middle of function calls or
function declarations, more or less, say, though some exceptions are around), the wrapping would be
unnecessary if they had a reasonable number of parameters and reasonably short variable names.
Long lines seem to go with long names, and getting rid of the long names helps a lot.\n\nIn other words, break lines because of the semantics of what you’re writing (as a general rule)
and not because of the length of the line. If you find that this produces lines that are too long,
then change the names or the semantics and you’ll probably get a good result.\n\nThis is, actually, exactly the same advice about how long a function should be. There’s no rule
“never have a function more than N lines long”, but there is definitely such a thing as too long
of a function, and of too repetitive tiny functions, and the solution is to change where the function
boundaries are, not to start counting lines.\n\n## Mixed Caps\n\nSee https://go.dev/doc/effective_go#mixed-caps. This applies even when it breaks conventions in other languages. For example an unexported constant is maxLength not MaxLength or MAX_LENGTH.\n\nAlso see Initialisms.\n\n## Named Result Parameters\n\nConsider what it will look like in godoc. Named result parameters like:\n\n```\nfunc (n *Node) Parent1() (node *Node) {}
func (n *Node) Parent2() (node *Node, err error) {}
\n```\n\nwill be repetitive in godoc; better to use:\n\n```\nfunc (n *Node) Parent1() *Node {}
func (n *Node) Parent2() (*Node, error) {}
\n```\n\nOn the other hand, if a function returns two or three parameters of the same type,
or if the meaning of a result isn’t clear from context, adding names may be useful
in some contexts. Don’t name result parameters just to avoid declaring a var inside
the function; that trades off a minor implementation brevity at the cost of
unnecessary API verbosity.\n\n```\nfunc (f *Foo) Location() (float64, float64, error)
\n```\n\nis less clear than:\n\n```\n// Location returns f's latitude and longitude.
// Negative values mean south and west, respectively.
func (f *Foo) Location() (lat, long float64, err error)
\n```\n\nNaked returns are okay if the function is a handful of lines. Once it’s a medium
sized function, be explicit with your return values. Corollary: it’s not worth it
to name result parameters just because it enables you to use naked returns.
Clarity of docs is always more important than saving a line or two in your function.\n\nFinally, in some cases you need to name a result parameter in order to change
it in a deferred closure. That is always OK.\n\n## Naked Returns\n\nA return statement without arguments returns the named return values. This is known as a “naked” return.\n\n```\nfunc split(sum int) (x, y int) {
    x = sum * 4 / 9
    y = sum - x
    return
}
\n```\n\nSee Named Result Parameters.\n\n## Package Comments\n\nPackage comments, like all comments to be presented by godoc, must appear adjacent to the package clause, with no blank line.\n\n```\n// Package math provides basic constants and mathematical functions.
package math
\n```\n\n```\n/*
Package template implements data-driven templates for generating textual
output such as HTML.
....
*/
package template
\n```\n\nFor “package main” comments, other styles of comment are fine after the binary name (and it may be capitalized if it comes first), For example, for a package main in the directory seedgen you could write:\n\n```\n// Binary seedgen ...
package main
\n```\n\nor\n\n```\n// Command seedgen ...
package main
\n```\n\nor\n\n```\n// Program seedgen ...
package main
\n```\n\nor\n\n```\n// The seedgen command ...
package main
\n```\n\nor\n\n```\n// The seedgen program ...
package main
\n```\n\nor\n\n```\n// Seedgen ..
package main
\n```\n\nThese are examples, and sensible variants of these are acceptable.\n\nNote that starting the sentence with a lower-case word is not among the
acceptable options for package comments, as these are publicly-visible and
should be written in proper English, including capitalizing the first word
of the sentence. When the binary name is the first word, capitalizing it is
required even though it does not strictly match the spelling of the
command-line invocation.\n\nSee https://go.dev/doc/effective_go#commentary for more information about commentary conventions.\n\n## Package Names\n\nAll references to names in your package will be done using the package name,
so you can omit that name from the identifiers. For example, if you are in package chubby,
you don’t need type ChubbyFile, which clients will write as chubby.ChubbyFile.
Instead, name the type File, which clients will write as chubby.File.
Avoid meaningless package names like util, common, misc, api, types, and interfaces. See https://go.dev/doc/effective_go#package-names and
https://go.dev/blog/package-names for more.\n\n## Pass Values\n\nDon’t pass pointers as function arguments just to save a few bytes. If a function refers to its argument x only as *x throughout, then the argument shouldn’t be a pointer. Common instances of this include passing a pointer to a string (*string) or a pointer to an interface value (*io.Reader). In both cases the value itself is a fixed size and can be passed directly. This advice does not apply to large structs, or even small structs that might grow.\n\n## Receiver Names\n\nThe name of a method’s receiver should be a reflection of its identity; often a one or two letter abbreviation of its type suffices (such as “c” or “cl” for “Client”). Don’t use generic names such as “me”, “this” or “self”, identifiers typical of object-oriented languages that gives the method a special meaning. In Go, the receiver of a method is just another parameter and therefore, should be named accordingly. The name need not be as descriptive as that of a method argument, as its role is obvious and serves no documentary purpose. It can be very short as it will appear on almost every line of every method of the type; familiarity admits brevity. Be consistent, too: if you call the receiver “c” in one method, don’t call it “cl” in another.\n\n## Receiver Type\n\nChoosing whether to use a value or pointer receiver on methods can be difficult, especially to new Go programmers. If in doubt, use a pointer, but there are times when a value receiver makes sense, usually for reasons of efficiency, such as for small unchanging structs or values of basic type. Some useful guidelines:\n\n- If the receiver is a map, func or chan, don’t use a pointer to them. If the receiver is a slice and the method doesn’t reslice or reallocate the slice, don’t use a pointer to it.\n- If the method needs to mutate the receiver, the receiver must be a pointer.\n- If the receiver is a struct that contains a sync.Mutex or similar synchronizing field, the receiver must be a pointer to avoid copying.\n- If the receiver is a large struct or array, a pointer receiver is more efficient. How large is large? Assume it’s equivalent to passing all its elements as arguments to the method. If that feels too large, it’s also too large for the receiver.\n- Can function or methods, either concurrently or when called from this method, be mutating the receiver? A value type creates a copy of the receiver when the method is invoked, so outside updates will not be applied to this receiver. If changes must be visible in the original receiver, the receiver must be a pointer.\n- If the receiver is a struct, array or slice and any of its elements is a pointer to something that might be mutating, prefer a pointer receiver, as it will make the intention clearer to the reader.\n- If the receiver is a small array or struct that is naturally a value type (for instance, something like the time.Time type), with no mutable fields and no pointers, or is just a simple basic type such as int or string, a value receiver makes sense. A value receiver can reduce the amount of garbage that can be generated; if a value is passed to a value method, an on-stack copy can be used instead of allocating on the heap. (The compiler tries to be smart about avoiding this allocation, but it can’t always succeed.) Don’t choose a value receiver type for this reason without profiling first.\n- Don’t mix receiver types. Choose either pointers or struct types for all available methods.\n- Finally, when in doubt, use a pointer receiver.\n\n## Synchronous Functions\n\nPrefer synchronous functions - functions which return their results directly or finish any callbacks or channel ops before returning - over asynchronous ones.\n\nSynchronous functions keep goroutines localized within a call, making it easier to reason about their lifetimes and avoid leaks and data races. They’re also easier to test: the caller can pass an input and check the output without the need for polling or synchronization.\n\nIf callers need more concurrency, they can add it easily by calling the function from a separate goroutine. But it is quite difficult - sometimes impossible - to remove unnecessary concurrency at the caller side.\n\n## Useful Test Failures\n\nTests should fail with helpful messages saying what was wrong, with what inputs, what was actually got, and what was expected. It may be tempting to write a bunch of assertFoo helpers, but be sure your helpers produce useful error messages. Assume that the person debugging your failing test is not you, and is not your team. A typical Go test fails like:\n\n```\nif got != tt.want {
    t.Errorf("Foo(%q) = %d; want %d", tt.in, got, tt.want) // or Fatalf, if test can't test anything more past this point
}
\n```\n\nNote that the order here is actual != expected, and the message uses that order too. Some test frameworks encourage writing these backwards: 0 != x, “expected 0, got x”, and so on. Go does not.\n\nIf that seems like a lot of typing, you may want to write a table-driven test.\n\nAnother common technique to disambiguate failing tests when using a test helper with different input is to wrap each caller with a different TestFoo function, so the test fails with that name:\n\n```\nfunc TestSingleValue(t *testing.T) { testHelper(t, []int{80}) }
func TestNoValues(t *testing.T)    { testHelper(t, []int{}) }
\n```\n\nIn any case, the onus is on you to fail with a helpful message to whoever’s debugging your code in the future.\n\n## Variable Names\n\nVariable names in Go should be short rather than long. This is especially true for local variables with limited scope. Prefer c to lineCount. Prefer i to sliceIndex.\n\nThe basic rule: the further from its declaration that a name is used, the more descriptive the name must be. For a method receiver, one or two letters is sufficient. Common variables such as loop indices and readers can be a single letter (i, r). More unusual things and global variables need more descriptive names.\n\nSee also the longer discussion in the Google Go Style Guide.\n\nThis content is part of the Go Wiki.