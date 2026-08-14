package attestation

import (
	"net/url"
	"strings"
	"testing"

	"github.com/windlasstech/slsa-builder/internal/identity"
)

func FuzzValidateSignerURI(f *testing.F) {
	base := "https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml"
	f.Add(base + "@refs/heads/main")
	f.Add(base + "@refs/tags/v1.0.0")
	f.Add("https://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yaml@refs/heads/main")
	f.Add(base + "@d2d916c6d6694c82c79d15c0393139b4084d4acc")
	f.Add("")
	f.Add("main")
	f.Add(base)
	f.Add(base + "@")
	f.Add(base + "@main")
	f.Add(base + "@d2d916c6")
	f.Add(base + "@D2D916C6D6694C82C79D15C0393139B4084D4ACC")
	f.Add(base + "@refs/heads/main@extra")
	f.Add(base + "@refs/heads/main ")
	f.Add(base + "@refs/heads/main?foo=bar")
	f.Add(base + "@refs/heads/main#frag")
	f.Add("http://github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main")
	f.Add("https://user@github.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main")
	f.Add("https://github.com/windlasstech/slsa-builder/actions/build.yml@refs/heads/main")
	f.Add("https://ghe.example.com/windlasstech/slsa-builder/.github/workflows/js-ts-npm-package-slsa3.yml@refs/heads/main")
	f.Fuzz(func(t *testing.T, raw string) {
		if err := validateSignerURI(raw); err != nil {
			return
		}
		parsed, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("accepted URI %q fails url.Parse: %v", raw, err)
		}
		if parsed.Scheme != "https" || parsed.Host != "github.com" || parsed.User != nil ||
			parsed.RawQuery != "" || parsed.Fragment != "" {
			t.Fatalf("accepted URI %q carries forbidden components: %#v", raw, parsed)
		}
		if strings.Count(raw, "@") != 1 {
			t.Fatalf("accepted URI %q does not carry exactly one @", raw)
		}
		_, ref, _ := strings.Cut(strings.TrimPrefix(parsed.Path, "/"), "@")
		if strings.TrimSpace(ref) != ref {
			t.Fatalf("accepted URI %q carries whitespace in the ref", raw)
		}
		if !strings.HasPrefix(ref, "refs/") && identity.ValidateFullSHA(ref) != nil {
			t.Fatalf("accepted URI %q ref %q is neither a full ref nor a 40-hex SHA pin", raw, ref)
		}
	})
}
