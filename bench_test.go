package guard

import (
	"context"
	"strconv"
	"testing"
	"time"
)

func BenchmarkGuardCheck(b *testing.B) {
	g := NewGuard()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			key := strconv.Itoa(i % 1000)
			if err := g.Check(key); err == nil {
				g.Release(key)
			}
		}
	})
}

func BenchmarkGuardTimeHandle(b *testing.B) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(time.Second))
	defer gt.Close()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			gt.Handle(i % 1000)
		}
	})
}

func BenchmarkGuardMutexLock(b *testing.B) {
	g := NewGuardMutex(time.Hour, 30*time.Second, time.Second)
	defer g.Close()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			m := g.GetLock(i % 1000)
			m.Lock()
			m.Unlock()
		}
	})
}

func BenchmarkGuardRWMutexLock(b *testing.B) {
	g := NewGuardRWMutex(time.Hour, 30*time.Second, time.Second)
	defer g.Close()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			m := g.GetRWLock(i % 1000)
			m.Lock()
			m.Unlock()
		}
	})
}

func BenchmarkGuardRWMutexRLock(b *testing.B) {
	g := NewGuardRWMutex(time.Hour, 30*time.Second, time.Second)
	defer g.Close()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			m := g.GetRWLock(i % 1000)
			m.RLock()
			m.RUnlock()
		}
	})
}

func BenchmarkGuardWaitAcquire(b *testing.B) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	bucket, _ := gw.GetSem("k", 1024)
	defer bucket.Release()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bucket.Acquire(context.Background(), 1)
			bucket.ReleaseTicket(1)
		}
	})
}

func BenchmarkGuardWaitAcquireContended(b *testing.B) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	bucket, _ := gw.GetSem("k", 1)
	defer bucket.Release()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bucket.Acquire(context.Background(), 1)
			bucket.ReleaseTicket(1)
		}
	})
}

func BenchmarkGuardWaitCondTicket(b *testing.B) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	defer g.Close()
	bucket, _ := g.GetBucket("k", 1024)
	defer bucket.Release()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.GotTicket()
			bucket.ReleaseTicket()
		}
	})
}

func BenchmarkGuardWaitCondTicketContended(b *testing.B) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	defer g.Close()
	bucket, _ := g.GetBucket("k", 1)
	defer bucket.Release()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.GotTicket()
			bucket.ReleaseTicket()
		}
	})
}

func BenchmarkGuardAcquire(b *testing.B) {
	g := NewGuard()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			key := strconv.Itoa(i % 1000)
			release, err := g.Acquire(key)
			if err == nil {
				release()
			}
		}
	})
}

func BenchmarkGuardMutexLockContended(b *testing.B) {
	g := NewGuardMutex(time.Hour, 30*time.Second, time.Second)
	defer g.Close()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := g.GetLock(1) // 固定 key，模拟高竞争
			m.Lock()
			m.Unlock()
		}
	})
}

func BenchmarkGuardRWMutexRLockContended(b *testing.B) {
	g := NewGuardRWMutex(time.Hour, 30*time.Second, time.Second)
	defer g.Close()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := g.GetRWLock(1) // 固定 key，模拟高竞争
			m.RLock()
			m.RUnlock()
		}
	})
}

func BenchmarkGuardWaitSemWeighted(b *testing.B) {
	gw := NewGuardWaitSem(time.Second, time.Minute)
	defer gw.Close()
	bucket, _ := gw.GetSem("k", 1024)
	defer bucket.Release()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bucket.Acquire(context.Background(), 4)
			bucket.ReleaseTicket(4)
		}
	})
}
