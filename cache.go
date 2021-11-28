package gateway

type cache struct {
	data     []byte
	expireAt int64
}
