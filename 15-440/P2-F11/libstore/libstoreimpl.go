package libstore

type Libstore struct {
}

func iNewLibstore(master, myhostport string, flags int) (*Libstore, error) {
	return nil, nil
}

func (ls *Libstore) iGet(key string) (string, error) {
	return "", nil
}

func (ls *Libstore) iPut(key, value string) error {
	return nil
}

func (ls *Libstore) iGetList(key string) ([]string, error) {
	return nil, nil
}

func (ls *Libstore) iRemoveFromList(key, removeitem string) error {
	return nil
}

func (ls *Libstore) iAppendToList(key, newitem string) error {
	return nil
}
