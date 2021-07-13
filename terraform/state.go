package terraform

type State struct {
	Data []byte
	Lock []byte
}
