package s3db

import (
	"fmt"
	"sync"
)

type FakeS3DB struct {
	objects map[string][]byte
	mutex   sync.Mutex
}

func NewFakeS3DB() *FakeS3DB {
	return &FakeS3DB{
		objects: make(map[string][]byte),
	}
}

func (f *FakeS3DB) FetchObject(key string) ([]byte, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if data, ok := f.objects[key]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("not found")
}

func (f *FakeS3DB) PutObject(key string, data []byte) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.objects[key] = data
	return nil
}
