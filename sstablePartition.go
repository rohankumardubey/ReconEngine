package reconEngine

import (
	"bufio"
	"bytes"
	"encoding/gob"
	"io"
	"log"
	"os"
	"path"
	"strconv"
)

const removed = 0x04

var BinDir = "bin"

type SsTablePartitionStorage interface {
	Get(key []byte) ([]byte, error)
	Set(key []byte, value []byte) error
	Del(key []byte) error
	Range(cb func(key []byte, value []byte) bool)
	CreatedAt() int64
	Close() error
}

type Index struct {
	Offset int64
	Length int32
}

// It's a part of ss table, loaded parts can be a readOnly
type ssTablePartition struct {
	createdAt int64
	index     map[string]Index
	fd        *os.File
}

func (ssp *ssTablePartition) Range(cb func(key []byte, value []byte) bool) {
	for k := range ssp.index {
		v, err := ssp.Get([]byte(k))
		if err != nil && err != KeyRemovedErr {
			continue
		}
		if !cb([]byte(k), v) {
			break
		}
	}
}

func (ssp *ssTablePartition) Get(key []byte) ([]byte, error) {
	index, ok := ssp.index[string(key)]
	if !ok {
		return nil, KeyNotFoundErr
	}

	_, err := ssp.fd.Seek(index.Offset, 0)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(ssp.fd)

	val, err := reader.Peek(int(index.Length))
	if err != nil {
		return nil, err
	}

	if bytes.Equal(val, []byte{removed}) {
		return val, KeyRemovedErr
	}
	return val, nil
}

func (ssp *ssTablePartition) Set(key []byte, value []byte) error {
	n, err := ssp.fd.Write(value)
	if err != nil {
		return err
	}
	fi, err := ssp.fd.Stat()
	if err != nil {
		return err
	}
	ssp.index[string(key)] = Index{
		Offset: fi.Size() - int64(n),
		Length: int32(len(value)),
	}
	return saveIndex(ssp.createdAt, ssp.index)
}

func (ssp *ssTablePartition) Del(key []byte) error {
	return ssp.Set(key, []byte{removed})
}

func (ssp *ssTablePartition) CreatedAt() int64 {
	return ssp.createdAt
}

func (ssp *ssTablePartition) Close() error {
	err := ssp.fd.Sync()
	if err != nil {
		return err
	}

	err = saveIndex(ssp.createdAt, ssp.index)
	if err != nil {
		return err
	}
	err = ssp.fd.Close()
	if err != nil {
		return err
	}
	return nil
}

func createIndex(createdAt int64) (index map[string]Index, err error) {
	fd, err := os.OpenFile(makePath("index", createdAt), os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return
	}
	fi, err := fd.Stat()
	if err != nil {
		return
	}

	if fi.Size() != 0 {
		err = gob.NewDecoder(fd).Decode(&index)
		return
	} else {
		index = make(map[string]Index)
		return
	}
}

func saveIndex(createdAt int64, index map[string]Index) error {
	fd, err := os.OpenFile(makePath("index", createdAt), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	err = gob.NewEncoder(fd).Encode(index)
	if err != nil {
		return err
	}

	return fd.Close()
}

func makePath(prefix string, createdAt int64) string {
	return path.Join(BinDir, strconv.FormatInt(createdAt, 10)+"-"+prefix+".bin")
}

func NewSStablePartition(createdAt int64) SsTablePartitionStorage {
	fd, err := os.OpenFile(makePath("partition", createdAt), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Panic(err)
	}
	index, err := createIndex(createdAt)
	if err != nil && err != io.EOF {
		log.Panic(err)
	}

	return &ssTablePartition{
		createdAt: createdAt,
		fd:        fd,
		index:     index,
	}
}
