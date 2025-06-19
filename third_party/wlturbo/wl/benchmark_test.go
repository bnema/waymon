package wl

import (
	"bytes"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"testing"
	
	"github.com/bnema/wlturbo"
)

// Benchmark event dispatching performance
func BenchmarkEventDispatch(b *testing.B) {
	// Create event dispatcher
	dispatcher := wlturbo.NewEventDispatcher()

	// Register handlers for different object IDs
	for i := uint32(1); i < 100; i++ {
		dispatcher.RegisterHandler(i, 0, func(e *wlturbo.Event) {
			// Minimal handler - just read the data
			if d := e.Data(); len(d) > 0 {
				_ = d[0]
			}
		})
	}

	// Create test data
	testData := make([]byte, 128)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dispatcher.Dispatch(50, 0, testData)
		}
	})
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N), "ns/dispatch")
}

// Benchmark event marshaling
func BenchmarkEventMarshal(b *testing.B) {
	objectID := uint32(42)
	opcode := uint16(5)
	data := make([]byte, 256)

	var buf bytes.Buffer
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		// Simulate Wayland protocol marshaling
		size := uint32(len(data) + 8)
		header1 := objectID
		header2 := size<<16 | uint32(opcode)
		
		binary.Write(&buf, binary.LittleEndian, header1)
		binary.Write(&buf, binary.LittleEndian, header2)
		buf.Write(data)
	}
	b.SetBytes(int64(len(data) + 8))
}

// Benchmark sync.Map vs sharded map
func BenchmarkSyncMapVsSharded(b *testing.B) {
	b.Run("sync.Map", func(b *testing.B) {
		var m sync.Map
		// Pre-populate
		for i := 0; i < 1000; i++ {
			m.Store(uint32(i), &wlturbo.BaseProxy{})
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := uint32(0)
			for pb.Next() {
				id := atomic.AddUint32(&i, 1) % 1000
				v, _ := m.Load(id)
				_ = v.(*wlturbo.BaseProxy)
			}
		})
	})

	b.Run("sharded-map", func(b *testing.B) {
		const shards = 16
		type shard struct {
			mu sync.RWMutex
			m  map[uint32]*wlturbo.BaseProxy
		}
		maps := make([]shard, shards)
		for i := range maps {
			maps[i].m = make(map[uint32]*wlturbo.BaseProxy)
		}

		// Pre-populate
		for i := uint32(0); i < 1000; i++ {
			s := &maps[i%shards]
			s.mu.Lock()
			s.m[i] = &wlturbo.BaseProxy{}
			s.mu.Unlock()
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := uint32(0)
			for pb.Next() {
				id := atomic.AddUint32(&i, 1) % 1000
				s := &maps[id%shards]
				s.mu.RLock()
				v := s.m[id]
				_ = v
				s.mu.RUnlock()
			}
		})
	})
}

// Benchmark string allocation vs interning
func BenchmarkStringHandling(b *testing.B) {
	data := []byte("wl_compositor\x00other data here")
	strlen := 14

	b.Run("allocation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			str := string(data[:strlen-1])
			_ = str
		}
	})

	b.Run("interning", func(b *testing.B) {
		// Simple string intern pool
		var internPool sync.Map
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			key := string(data[:strlen-1])
			if cached, ok := internPool.Load(key); ok {
				_ = cached.(string)
			} else {
				internPool.Store(key, key)
			}
		}
	})
}

// Benchmark buffer pooling strategies
func BenchmarkBufferPools(b *testing.B) {
	b.Run("bytes.Buffer", func(b *testing.B) {
		pool := &sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		}
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := pool.Get().(*bytes.Buffer)
				buf.Write(make([]byte, 256))
				buf.Reset()
				pool.Put(buf)
			}
		})
	})

	b.Run("byte-slice", func(b *testing.B) {
		pool := &sync.Pool{
			New: func() interface{} {
				b := make([]byte, 0, 512)
				return &b
			},
		}
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				bufPtr := pool.Get().(*[]byte)
				buf := (*bufPtr)[:0]
				buf = append(buf, make([]byte, 256)...)
				pool.Put(&buf)
			}
		})
	})
}

// Benchmark event allocation from pool
func BenchmarkEventPool(b *testing.B) {
	// Use the global event pool from wlturbo package
	b.Run("pool-alloc", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// Simulate what happens in Dispatch
				event := &wlturbo.Event{
					ProxyID: 42,
					Opcode: 1,
				}
				_ = event
			}
		})
	})

	b.Run("direct-alloc", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				event := &wlturbo.Event{
					ProxyID: 42,
					Opcode: 1,
				}
				_ = event
			}
		})
	})
}

// Benchmark high-frequency pointer motion events
func BenchmarkPointerMotion(b *testing.B) {
	dispatcher := wlturbo.NewEventDispatcher()
	
	// Register motion handler
	var totalX, totalY int32
	dispatcher.RegisterHandler(10, 4, func(e *wlturbo.Event) { // motion opcode = 4
		// Simulate reading fixed-point coordinates
		d := e.Data()
		if len(d) >= 16 {
			x := int32(binary.LittleEndian.Uint32(d[4:8]))
			y := int32(binary.LittleEndian.Uint32(d[8:12]))
			atomic.AddInt32(&totalX, x>>8)
			atomic.AddInt32(&totalY, y>>8)
		}
	})

	// Pre-create motion event data
	eventData := make([]byte, 16) // timestamp(4) + x(4) + y(4) + padding
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Update coordinates for each event
			binary.LittleEndian.PutUint32(eventData[4:], uint32(i*256)) // x as wl_fixed
			binary.LittleEndian.PutUint32(eventData[8:], uint32(i*256)) // y as wl_fixed
			
			dispatcher.Dispatch(10, 4, eventData)
			i++
		}
	})
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N), "ns/motion")
}

// Benchmark concurrent message sending
func BenchmarkConcurrentSend(b *testing.B) {
	// Simple mutex vs atomic benchmark for send path
	var mu sync.Mutex
	sendBuf := make([]byte, 0, 4096)
	msg := bytes.Repeat([]byte("test"), 32) // 128 byte message

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Simulate send locking pattern
			mu.Lock()
			sendBuf = append(sendBuf[:0], msg...)
			mu.Unlock()
		}
	})
	b.SetBytes(int64(len(msg)))
}

// Benchmark atomic operations for FD queue
func BenchmarkAtomicOps(b *testing.B) {
	var counter atomic.Int64

	b.Run("atomic-add", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				counter.Add(1)
			}
		})
	})

	b.Run("cas-loop", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				for {
					old := counter.Load()
					if counter.CompareAndSwap(old, old+1) {
						break
					}
				}
			}
		})
	})
}

// Benchmark different dispatch strategies
func BenchmarkDispatchStrategies(b *testing.B) {
	b.Run("switch-statement", func(b *testing.B) {
		handlers := make(map[uint8]func())
		for i := uint8(0); i < 32; i++ {
			i := i
			handlers[i] = func() { _ = i }
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			opcode := uint8(i % 32)
			switch opcode {
			case 0: handlers[0]()
			case 1: handlers[1]()
			case 2: handlers[2]()
			case 3: handlers[3]()
			case 4: handlers[4]()
			default:
				if h, ok := handlers[opcode]; ok {
					h()
				}
			}
		}
	})

	b.Run("array-lookup", func(b *testing.B) {
		var handlers [32]func()
		for i := range handlers {
			i := i
			handlers[i] = func() { _ = i }
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			opcode := uint8(i % 32)
			if opcode < 32 && handlers[opcode] != nil {
				handlers[opcode]()
			}
		}
	})

	b.Run("map-lookup", func(b *testing.B) {
		handlers := make(map[uint8]func())
		for i := uint8(0); i < 32; i++ {
			i := i
			handlers[i] = func() { _ = i }
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			opcode := uint8(i % 32)
			if h, ok := handlers[opcode]; ok {
				h()
			}
		}
	})
}

// Benchmark zero-copy vs copy strategies
func BenchmarkZeroCopyVsCopy(b *testing.B) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}

	b.Run("copy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Simulate copying data for each event
			eventData := make([]byte, len(data))
			copy(eventData, data)
			_ = eventData
		}
		b.SetBytes(int64(len(data)))
	})

	b.Run("zero-copy", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// Simulate zero-copy by just referencing data
			eventData := data
			_ = eventData
		}
		b.SetBytes(int64(len(data)))
	})
}