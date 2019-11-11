package reconEngine

import (
	"bytes"
	"os"
	"sync"
)

// Merge sort algoritm (merge old partitions in bigger one)
func (ssTable *ssTable) MergeSort() error {
	var mx sync.Mutex
	mx.Lock()
	defer mx.Unlock()
	if len(ssTable.GetAvailablePartitions())+len(ssTable.OpenedPartitions) <= 1 {
		return nil
	}
	values := make(map[string][]byte)
	ssTable.Range(func(createdAt int64, partitionStorage SsTablePartitionStorage) bool {
		partitionStorage.Range(func(k []byte, v []byte) bool {
			values[string(k)] = v
			return true
		})
		return true
	})
	ssp := ssTable.CreatePartition()
	for k, v := range values {
		if !bytes.Equal(v, []byte{removed}) {
			err := ssp.Set([]byte(k), v)
			if err != nil {
				return err
			}
		}
	}

	err := ssTable.CloseAll()
	if err != nil {
		return err
	} else {
		for _, c := range ssTable.AvailablePartitions {
			if c == ssp.CreatedAt() {
				continue
			}
			err := os.Remove(makePath("partition", c))
			if err != nil {
				return err
			}
			err = os.Remove(makePath("index", c))
			if err != nil {
				return err
			}
		}
	}
	ssTable.AvailablePartitions = make(ssTablePartitionKeys, 0)
	ssTable.OpenedPartitions = ssTablePartitions{ssp}
	return nil
}
