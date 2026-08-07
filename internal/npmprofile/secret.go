package npmprofile

const redactedSecret = "[REDACTED]"

// SecretToken holds a credential in memory while preventing accidental formatting disclosure.
type SecretToken struct {
	secret string
}

func newSecretToken(value string) SecretToken {
	return SecretToken{secret: value}
}

func (token SecretToken) valid() bool {
	return token.secret != ""
}

func (token SecretToken) value() string {
	return token.secret
}

// String redacts the credential from ordinary formatting and errors.
func (SecretToken) String() string {
	return redactedSecret
}

// GoString redacts the credential from %#v formatting.
func (SecretToken) GoString() string {
	return "npmprofile.SecretToken{" + redactedSecret + "}"
}
