package poco

type PocoConnection interface {
	Connect() error
	SendAndReceive(data []byte) ([]byte, error)
	Disconnect()
	IsConnected() bool
}
