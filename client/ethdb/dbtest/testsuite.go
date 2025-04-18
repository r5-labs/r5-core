// Copyright 2025 R5 Labs
// This file is part of the R5 Core library.
//
// This software is provided "as is", without warranty of any kind,
// express or implied, including but not limited to the warranties
// of merchantability, fitness for a particular purpose and
// noninfringement. In no event shall the authors or copyright
// holders be liable for any claim, damages, or other liability,
// whether in an action of contract, tort or otherwise, arising
// from, out of or in connection with the software or the use or
// other dealings in the software.

package dbtest

import (
	"bytes"
	"crypto/rand"
	"reflect"
	"sort"
	"testing"

	"github.com/r5-labs/r5-core/client/ethdb"
)

// TestDatabaseSuite runs a suite of tests against a KeyValueStore database
// implementation.
func TestDatabaseSuite(t *testing.T, New func() ethdb.KeyValueStore) {
	t.Run("Iterator", func(t *testing.T) {
		tests := []struct {
			content map[string]string
			prefix  string
			start   string
			order   []string
		}{
			// Empty databases should be iterable
			{map[string]string{}, "", "", nil},
			{map[string]string{}, "non-existent-prefix", "", nil},

			// Single-item databases should be iterable
			{map[string]string{"key": "val"}, "", "", []string{"key"}},
			{map[string]string{"key": "val"}, "k", "", []string{"key"}},
			{map[string]string{"key": "val"}, "l", "", nil},

			// Multi-item databases should be fully iterable
			{
				map[string]string{"k1": "v1", "k5": "v5", "k2": "v2", "k4": "v4", "k3": "v3"},
				"", "",
				[]string{"k1", "k2", "k3", "k4", "k5"},
			},
			{
				map[string]string{"k1": "v1", "k5": "v5", "k2": "v2", "k4": "v4", "k3": "v3"},
				"k", "",
				[]string{"k1", "k2", "k3", "k4", "k5"},
			},
			{
				map[string]string{"k1": "v1", "k5": "v5", "k2": "v2", "k4": "v4", "k3": "v3"},
				"l", "",
				nil,
			},
			// Multi-item databases should be prefix-iterable
			{
				map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				"ka", "",
				[]string{"ka1", "ka2", "ka3", "ka4", "ka5"},
			},
			{
				map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				"kc", "",
				nil,
			},
			// Multi-item databases should be prefix-iterable with start position
			{
				map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				"ka", "3",
				[]string{"ka3", "ka4", "ka5"},
			},
			{
				map[string]string{
					"ka1": "va1", "ka5": "va5", "ka2": "va2", "ka4": "va4", "ka3": "va3",
					"kb1": "vb1", "kb5": "vb5", "kb2": "vb2", "kb4": "vb4", "kb3": "vb3",
				},
				"ka", "8",
				nil,
			},
		}
		for i, tt := range tests {
			// Create the key-value data store
			db := New()
			for key, val := range tt.content {
				if err := db.Put([]byte(key), []byte(val)); err != nil {
					t.Fatalf("test %d: failed to insert item %s:%s into database: %v", i, key, val, err)
				}
			}
			// Iterate over the database with the given configs and verify the results
			it, idx := db.NewIterator([]byte(tt.prefix), []byte(tt.start)), 0
			for it.Next() {
				if len(tt.order) <= idx {
					t.Errorf("test %d: prefix=%q more items than expected: checking idx=%d (key %q), expecting len=%d", i, tt.prefix, idx, it.Key(), len(tt.order))
					break
				}
				if !bytes.Equal(it.Key(), []byte(tt.order[idx])) {
					t.Errorf("test %d: item %d: key mismatch: have %s, want %s", i, idx, string(it.Key()), tt.order[idx])
				}
				if !bytes.Equal(it.Value(), []byte(tt.content[tt.order[idx]])) {
					t.Errorf("test %d: item %d: value mismatch: have %s, want %s", i, idx, string(it.Value()), tt.content[tt.order[idx]])
				}
				idx++
			}
			if err := it.Error(); err != nil {
				t.Errorf("test %d: iteration failed: %v", i, err)
			}
			if idx != len(tt.order) {
				t.Errorf("test %d: iteration terminated prematurely: have %d, want %d", i, idx, len(tt.order))
			}
			db.Close()
		}
	})

	t.Run("IteratorWith", func(t *testing.T) {
		db := New()
		defer db.Close()

		keys := []string{"1", "2", "3", "4", "6", "10", "11", "12", "20", "21", "22"}
		sort.Strings(keys) // 1, 10, 11, etc

		for _, k := range keys {
			if err := db.Put([]byte(k), nil); err != nil {
				t.Fatal(err)
			}
		}

		{
			it := db.NewIterator(nil, nil)
			got, want := iterateKeys(it), keys
			if err := it.Error(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Iterator: got: %s; want: %s", got, want)
			}
		}

		{
			it := db.NewIterator([]byte("1"), nil)
			got, want := iterateKeys(it), []string{"1", "10", "11", "12"}
			if err := it.Error(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("IteratorWith(1,nil): got: %s; want: %s", got, want)
			}
		}

		{
			it := db.NewIterator([]byte("5"), nil)
			got, want := iterateKeys(it), []string{}
			if err := it.Error(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("IteratorWith(5,nil): got: %s; want: %s", got, want)
			}
		}

		{
			it := db.NewIterator(nil, []byte("2"))
			got, want := iterateKeys(it), []string{"2", "20", "21", "22", "3", "4", "6"}
			if err := it.Error(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("IteratorWith(nil,2): got: %s; want: %s", got, want)
			}
		}

		{
			it := db.NewIterator(nil, []byte("5"))
			got, want := iterateKeys(it), []string{"6"}
			if err := it.Error(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("IteratorWith(nil,5): got: %s; want: %s", got, want)
			}
		}
	})

	t.Run("KeyValueOperations", func(t *testing.T) {
		db := New()
		defer db.Close()

		key := []byte("foo")

		if got, err := db.Has(key); err != nil {
			t.Error(err)
		} else if got {
			t.Errorf("wrong value: %t", got)
		}

		value := []byte("hello world")
		if err := db.Put(key, value); err != nil {
			t.Error(err)
		}

		if got, err := db.Has(key); err != nil {
			t.Error(err)
		} else if !got {
			t.Errorf("wrong value: %t", got)
		}

		if got, err := db.Get(key); err != nil {
			t.Error(err)
		} else if !bytes.Equal(got, value) {
			t.Errorf("wrong value: %q", got)
		}

		if err := db.Delete(key); err != nil {
			t.Error(err)
		}

		if got, err := db.Has(key); err != nil {
			t.Error(err)
		} else if got {
			t.Errorf("wrong value: %t", got)
		}
	})

	t.Run("Batch", func(t *testing.T) {
		db := New()
		defer db.Close()

		b := db.NewBatch()
		for _, k := range []string{"1", "2", "3", "4"} {
			if err := b.Put([]byte(k), nil); err != nil {
				t.Fatal(err)
			}
		}

		if has, err := db.Has([]byte("1")); err != nil {
			t.Fatal(err)
		} else if has {
			t.Error("db contains element before batch write")
		}

		if err := b.Write(); err != nil {
			t.Fatal(err)
		}

		{
			it := db.NewIterator(nil, nil)
			if got, want := iterateKeys(it), []string{"1", "2", "3", "4"}; !reflect.DeepEqual(got, want) {
				t.Errorf("got: %s; want: %s", got, want)
			}
		}

		b.Reset()

		// Mix writes and deletes in batch
		b.Put([]byte("5"), nil)
		b.Delete([]byte("1"))
		b.Put([]byte("6"), nil)
		b.Delete([]byte("3"))
		b.Put([]byte("3"), nil)

		if err := b.Write(); err != nil {
			t.Fatal(err)
		}

		{
			it := db.NewIterator(nil, nil)
			if got, want := iterateKeys(it), []string{"2", "3", "4", "5", "6"}; !reflect.DeepEqual(got, want) {
				t.Errorf("got: %s; want: %s", got, want)
			}
		}
	})

	t.Run("BatchReplay", func(t *testing.T) {
		db := New()
		defer db.Close()

		want := []string{"1", "2", "3", "4"}
		b := db.NewBatch()
		for _, k := range want {
			if err := b.Put([]byte(k), nil); err != nil {
				t.Fatal(err)
			}
		}

		b2 := db.NewBatch()
		if err := b.Replay(b2); err != nil {
			t.Fatal(err)
		}

		if err := b2.Replay(db); err != nil {
			t.Fatal(err)
		}

		it := db.NewIterator(nil, nil)
		if got := iterateKeys(it); !reflect.DeepEqual(got, want) {
			t.Errorf("got: %s; want: %s", got, want)
		}
	})

	t.Run("Snapshot", func(t *testing.T) {
		db := New()
		defer db.Close()

		initial := map[string]string{
			"k1": "v1", "k2": "v2", "k3": "", "k4": "",
		}
		for k, v := range initial {
			db.Put([]byte(k), []byte(v))
		}
		snapshot, err := db.NewSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range initial {
			got, err := snapshot.Get([]byte(k))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, []byte(v)) {
				t.Fatalf("Unexpected value want: %v, got %v", v, got)
			}
		}

		// Flush more modifications into the database, ensure the snapshot
		// isn't affected.
		var (
			update = map[string]string{"k1": "v1-b", "k3": "v3-b"}
			insert = map[string]string{"k5": "v5-b"}
			delete = map[string]string{"k2": ""}
		)
		for k, v := range update {
			db.Put([]byte(k), []byte(v))
		}
		for k, v := range insert {
			db.Put([]byte(k), []byte(v))
		}
		for k := range delete {
			db.Delete([]byte(k))
		}
		for k, v := range initial {
			got, err := snapshot.Get([]byte(k))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, []byte(v)) {
				t.Fatalf("Unexpected value want: %v, got %v", v, got)
			}
		}
		for k := range insert {
			got, err := snapshot.Get([]byte(k))
			if err == nil || len(got) != 0 {
				t.Fatal("Unexpected value")
			}
		}
		for k := range delete {
			got, err := snapshot.Get([]byte(k))
			if err != nil || len(got) == 0 {
				t.Fatal("Unexpected deletion")
			}
		}
	})
}

// BenchDatabaseSuite runs a suite of benchmarks against a KeyValueStore database
// implementation.
func BenchDatabaseSuite(b *testing.B, New func() ethdb.KeyValueStore) {
	var (
		keys, vals   = makeDataset(1_000_000, 32, 32, false)
		sKeys, sVals = makeDataset(1_000_000, 32, 32, true)
	)
	// Run benchmarks sequentially
	b.Run("Write", func(b *testing.B) {
		benchWrite := func(b *testing.B, keys, vals [][]byte) {
			b.ResetTimer()
			b.ReportAllocs()

			db := New()
			defer db.Close()

			for i := 0; i < len(keys); i++ {
				db.Put(keys[i], vals[i])
			}
		}
		b.Run("WriteSorted", func(b *testing.B) {
			benchWrite(b, sKeys, sVals)
		})
		b.Run("WriteRandom", func(b *testing.B) {
			benchWrite(b, keys, vals)
		})
	})
	b.Run("Read", func(b *testing.B) {
		benchRead := func(b *testing.B, keys, vals [][]byte) {
			db := New()
			defer db.Close()

			for i := 0; i < len(keys); i++ {
				db.Put(keys[i], vals[i])
			}
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < len(keys); i++ {
				db.Get(keys[i])
			}
		}
		b.Run("ReadSorted", func(b *testing.B) {
			benchRead(b, sKeys, sVals)
		})
		b.Run("ReadRandom", func(b *testing.B) {
			benchRead(b, keys, vals)
		})
	})
	b.Run("Iteration", func(b *testing.B) {
		benchIteration := func(b *testing.B, keys, vals [][]byte) {
			db := New()
			defer db.Close()

			for i := 0; i < len(keys); i++ {
				db.Put(keys[i], vals[i])
			}
			b.ResetTimer()
			b.ReportAllocs()

			it := db.NewIterator(nil, nil)
			for it.Next() {
			}
			it.Release()
		}
		b.Run("IterationSorted", func(b *testing.B) {
			benchIteration(b, sKeys, sVals)
		})
		b.Run("IterationRandom", func(b *testing.B) {
			benchIteration(b, keys, vals)
		})
	})
	b.Run("BatchWrite", func(b *testing.B) {
		benchBatchWrite := func(b *testing.B, keys, vals [][]byte) {
			b.ResetTimer()
			b.ReportAllocs()

			db := New()
			defer db.Close()

			batch := db.NewBatch()
			for i := 0; i < len(keys); i++ {
				batch.Put(keys[i], vals[i])
			}
			batch.Write()
		}
		b.Run("BenchWriteSorted", func(b *testing.B) {
			benchBatchWrite(b, sKeys, sVals)
		})
		b.Run("BenchWriteRandom", func(b *testing.B) {
			benchBatchWrite(b, keys, vals)
		})
	})
}

func iterateKeys(it ethdb.Iterator) []string {
	keys := []string{}
	for it.Next() {
		keys = append(keys, string(it.Key()))
	}
	sort.Strings(keys)
	it.Release()
	return keys
}

// randomHash generates a random blob of data and returns it as a hash.
func randBytes(len int) []byte {
	buf := make([]byte, len)
	if n, err := rand.Read(buf); n != len || err != nil {
		panic(err)
	}
	return buf
}

func makeDataset(size, ksize, vsize int, order bool) ([][]byte, [][]byte) {
	var keys [][]byte
	var vals [][]byte
	for i := 0; i < size; i += 1 {
		keys = append(keys, randBytes(ksize))
		vals = append(vals, randBytes(vsize))
	}
	if order {
		sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i], keys[j]) < 0 })
	}
	return keys, vals
}
