package secrets

type ConfigFile struct {
	EncryptedPath string
	DecryptedPath string
	Content       []byte
}

type EnvFile struct {
	EncryptedPath string
	DecryptedPath string
	Content       []byte
}
