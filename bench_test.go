package guard

import (
	"context"
	"testing"
	"time"
)

func BenchmarkGuardCheck(b *testing.B) {
	g := NewGuard()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			key := i % 1000
			if err := g.Check(key); err == nil {
				g.Release(key)
			}
		}
	})
}

func BenchmarkGuardTimeHandle(b *testing.B) {
	gt := NewGuardTime(OptionsGuardtime().SetInterval(time.Second))
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			gt.Handle(i % 1000)
		}
	})
}

func BenchmarkGuardMutexLock(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			m := GetLock(i % 1000)
			m.Lock()
			m.Unlock()
		}
	})
}

func BenchmarkGuardRWMutexLock(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			m := GetRWLock(i % 1000)
			m.Lock()
			m.Unlock()
		}
	})
}

func BenchmarkGuardRWMutexRLock(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			m := GetRWLock(i % 1000)
			m.RLock()
			m.RUnlock()
		}
	})
}

func BenchmarkGuardWaitAcquire(b *testing.B) {
	gw := NewGuardWait(time.Second, time.Minute)
	defer gw.Close()
	bucket, _ := gw.GetBucket("k", 1024)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bucket.Acquire(context.Background())
			bucket.Release()
		}
	})
}

func BenchmarkGuardWaitAcquireContended(b *testing.B) {
	gw := NewGuardWait(time.Second, time.Minute)
	defer gw.Close()
	bucket, _ := gw.GetBucket("k", 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = bucket.Acquire(context.Background())
			bucket.Release()
		}
	})
}

func BenchmarkGuardWaitCondTicket(b *testing.B) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	bucket := g.GetBucket("k", 1024)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.GotTicket()
			bucket.ReleaseTicket()
		}
	})
}

func BenchmarkGuardWaitCondTicketContended(b *testing.B) {
	g := NewGuardWaitCond(time.Second, time.Minute)
	bucket := g.GetBucket("k", 1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bucket.GotTicket()
			bucket.ReleaseTicket()
		}
	})
}
