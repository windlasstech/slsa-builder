package npmprofile

import "strings"

type builderContractEdit struct {
	descriptor int
	field      string
	value      string
}

func builderContractEdits(mutation string) []builderContractEdit {
	target := 1
	if strings.HasPrefix(mutation, "builder-signer-") {
		target = 0
	}
	field := ""
	var values []string
	switch mutation {
	case "builder-annotations", "builder-signer-annotations":
		field = "annotations"
		values = []string{"null", "false", "0", `"untrusted"`, `[]`, `["untrusted"]`, `{"digest":{"dirHash1":"0000000000000000000000000000000000000000000000000000000000000000"},"digest_authority":"untrusted","name":"forged-signing-adapter","uri":"pkg:golang/example.com/forged@v0.0.0"}`}
		if target == 1 {
			values[len(values)-1] = `{"digest_authority":"untrusted","other":true}`
			values = append([]string{""}, values...)
		}
	case "builder-empty":
		var edits []builderContractEdit
		for _, descriptor := range []int{0, 1} {
			for _, member := range []string{"extra", "downloadLocation", "content"} {
				for _, value := range []string{"null", `""`, "[]", "{}"} {
					edits = append(edits, builderContractEdit{descriptor, member, value})
				}
			}
		}
		for _, value := range []string{"null", `""`, "[]", "{}"} {
			edits = append(edits, builderContractEdit{0, "annotations", value})
		}
		return edits
	case "builder-publish-malformed", "builder-signer-malformed":
		var edits []builderContractEdit
		for _, member := range []string{"name", "uri", "digest"} {
			for _, value := range []string{"", "null", `""`, "{}", "[]", "false", "42", `{"sha512":42}`} {
				edits = append(edits, builderContractEdit{target, member, value})
			}
		}
		return edits
	case "builder-publish-misnamed", "builder-signer-misnamed":
		field, values = "name", []string{"", `"renamed"`, `"INVALID_NAME"`}
	case "builder-publish-version":
		field, values = "uri", []string{`"pkg:npm/npm@0.0.0"`}
	case "builder-signer-version":
		field, values = "uri", []string{`"pkg:golang/github.com/sigstore/sigstore-go@v0.0.0"`}
	case "builder-digest", "builder-signer-digest":
		algorithm, length := "sha512", 128
		if target == 0 {
			algorithm, length = "dirHash1", 64
		}
		field = "digest"
		for _, value := range []string{strings.Repeat("0", length), strings.Repeat("A", length), "ab", strings.Repeat("z", length)} {
			values = append(values, `{"`+algorithm+`":"`+value+`"}`)
		}
		values = append(values, `{"sha256":"`+strings.Repeat("a", 64)+`"}`, `{"`+algorithm+`":"`+strings.Repeat("a", length)+`","extra":"ab"}`)
	case "builder-publish-media":
		field, values = "mediaType", []string{"", "null", `""`, `"application/json"`, "42", "{}"}
	case "builder-extra", "builder-signer-extra":
		var edits []builderContractEdit
		members := []string{"extra", "downloadLocation", "content"}
		if target == 0 {
			members = append(members, "mediaType")
		}
		for _, member := range members {
			for _, value := range []string{"false", "0", `" "`, `"value"`, `[1]`, `{"key":true}`} {
				edits = append(edits, builderContractEdit{target, member, value})
			}
		}
		return edits
	case "builder-duplicate-json":
		return []builderContractEdit{
			{0, "annotations", `{"nested":{"key":1,"key":2}}`},
			{1, "annotations", `{"nested":[{"key":1,"key":2}]}`},
			{0, "digest", `{"dirHash1":"bad","dirHash1":"also-bad"}`},
			{1, "digest", `{"sha512":"bad","sha512":"also-bad"}`},
		}
	default:
		return []builderContractEdit{{}}
	}
	var edits []builderContractEdit
	for _, value := range values {
		edits = append(edits, builderContractEdit{target, field, value})
	}
	return edits
}
