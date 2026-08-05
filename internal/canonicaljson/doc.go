// Package canonicaljson validates, compares, and canonicalizes security-relevant JSON values.
//
// The package treats duplicate object members as a parser-differential threat: a signature proves
// the integrity of serialized bytes, but permissive parsers can still disagree about which repeated
// value a member represents. Validate therefore walks the decoder's token stream before any lossy
// map or struct decoding. Object member names are compared after JSON string unescaping at every
// nesting depth, including objects inside arrays. A duplicate fails closed with
// DuplicateJSONMemberID before semantic validation or RFC 8785 canonicalization begins.
//
// Callers must pass the original JSON bytes to this package. Validating a value after another parser
// has normalized it cannot prove that the signed or digest-bound input was unambiguous.
package canonicaljson
