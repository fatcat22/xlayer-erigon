package db

import "github.com/benbjohnson/immutable"

func (m *MemDb) GetLastHeight() (uint64, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	return m.LastHeight, nil
}

func (m *MemDb) SetLastHeight(value uint64) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.LastHeight = value
	return nil
}

func (m *MemDb) SetCache(*immutable.Map[string, *immutable.Map[string, []byte]]) {}

func (m *MemDb) RetriveAndCleanCache() map[string]map[string][]byte {
	return nil
}
