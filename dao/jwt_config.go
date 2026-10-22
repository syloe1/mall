package dao

type DefaultJWTConfig struct {
	Secret []byte
}

func (d *DefaultJWTConfig) GetSecret() []byte {
	return d.Secret
}
