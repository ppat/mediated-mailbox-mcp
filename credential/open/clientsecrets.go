package open

import "github.com/ppat/mediated-mailbox-mcp/credential/seal"

// ClientSecrets opens an OAuth client's secret, bound to the client-secret purpose and the client's
// row, and nothing else (ADR-0081). It holds the keyring and exposes no way to seal, to re-seal or to
// open under another context, so a value sealed as an account's credential, or as another client's
// secret, fails to open through it (ADR-0088). The UI's client-secret package holds one in place of
// the keyring.
type ClientSecrets struct {
	ring *Keyring
}

// LoadClientSecrets loads the keyring as Load does and returns the opening of client secrets over it.
func LoadClientSecrets(publicKeyPath string, privateKeyPaths ...string) (*ClientSecrets, error) {
	ring, err := Load(publicKeyPath, privateKeyPaths...)
	if err != nil {
		return nil, err
	}
	return &ClientSecrets{ring: ring}, nil
}

// Open opens sealed as the named client's secret.
func (c *ClientSecrets) Open(sealed []byte, client string) ([]byte, error) {
	return c.ring.Open(sealed, seal.ClientSecret(client))
}
