# RFC 8785 JCS Test Vectors

The `input/` and `output/` directories are the complete paired canonicalization vector set copied
without semantic changes from the RFC 8785 editor's reference implementation:

- Repository:
  [`cyberphone/json-canonicalization`](https://github.com/cyberphone/json-canonicalization)
- Source commit:
  [`19d51d7fe467d4706a3ff08adf8a748f29fc21e0`](https://github.com/cyberphone/json-canonicalization/tree/19d51d7fe467d4706a3ff08adf8a748f29fc21e0/testdata)
- Go module version: `v0.0.0-20241213102144-19d51d7fe467`
- Specification: [RFC 8785, JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785)
- Upstream license:
  [Apache License 2.0](https://github.com/cyberphone/json-canonicalization/blob/19d51d7fe467d4706a3ff08adf8a748f29fc21e0/LICENSE)

The six pairs cover arrays, locale-independent ordering, nested structures, preserved Unicode,
primitive values inside an object, strings, numbers, and UTF-16 property ordering. The upstream
`outhex/` directory is a hexadecimal rendering of these same expected output bytes and is not a
separate vector set, so it is not duplicated here.
