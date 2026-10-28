package core

type ProcessInfo struct {
	PID  int    `json:"pid"`
	Path string `json:"path"`
	Base uint64 `json:"base"`
	Size uint64 `json:"size"`
}
type Memory interface {
	Info() ProcessInfo
	Read(uint64, int) ([]byte, error)
	Write(uint64, []byte) error
	Pause() (func() error, error)
	Alive() bool
	Close() error
}
