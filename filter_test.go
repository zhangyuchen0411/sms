package sms

import (
	"github.com/garyburd/redigo/redis"
	"github.com/stretchr/testify/assert"
	"github.com/uber-go/zap"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func buildTestRedisPool() *redis.Pool {
	pool := &redis.Pool{
		MaxIdle:     10,
		MaxActive:   50,
		IdleTimeout: 1 * time.Second,
		Dial: func() (c redis.Conn, err error) {
			c, err = redis.DialTimeout("tcp", "127.0.0.1:6379", 5*time.Second, 3*time.Second, 5*time.Second)
			if err != nil {
				return
			}
			return
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
		Wait: false,
	}
	return pool
}

// RateLimitFilterRedis

func TestRateLimitFilterRedis_Filter(t *testing.T) {
	pool := buildTestRedisPool()
	defer pool.Close()

	filter := NewRateLimitFilterRedis(pool, 3, 1, 3*time.Second)
	filter.KeyExpireSec = 5

	ctx := &Context{
		Logger: zap.NewJSON(),
	}
	ctx.Logger.SetLevel(zap.DebugLevel)

	sucCount := 0
	for i := 0; i < 5; i++ {
		req := &SMSReq{
			PhoneNumbers: []string{"123"},
		}
		pns, failed := filter.Filter(ctx, req)
		if len(failed) == 0 {
			sucCount++
		}
		assert.Equal(t, 1, len(pns)+len(failed))
	}

	assert.Equal(t, 3, sucCount)
}

func TestRateLimitFilterRedis_Concurrence(t *testing.T) {
	pool := buildTestRedisPool()
	defer pool.Close()

	filter := NewRateLimitFilterRedis(pool, 3, 3, time.Second)
	filter.KeyExpireSec = 3

	sucCount := int64(0)
	wg := &sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			ctx := &Context{
				Logger: zap.NewJSON().With(zap.Int("index", i)),
			}
			ctx.Logger.SetLevel(zap.InfoLevel)

			for i := 0; i < 500; i++ {
				req := &SMSReq{
					PhoneNumbers: []string{"124"},
				}
				_, failed := filter.Filter(ctx, req)
				if len(failed) == 0 {
					atomic.AddInt64(&sucCount, 1)
				}
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(3), sucCount)
}

// 116709 ns/op
func BenchmarkRateLimitFilterRedis_Filter(b *testing.B) {
	pool := buildTestRedisPool()
	defer pool.Close()

	filter := NewRateLimitFilterRedis(pool, 1, 1, time.Second)
	ctx := &Context{
		Logger: zap.NewJSON(),
	}
	ctx.Logger.SetLevel(zap.InfoLevel)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := &SMSReq{
			PhoneNumbers: []string{"125"},
		}
		filter.Filter(ctx, req)
	}
}

// RateLimitFilterRedisCounter

func TestRateLimitFilterRedisCounter_Filter(t *testing.T) {
	pool := buildTestRedisPool()
	defer pool.Close()

	filter := NewRateLimitFilterRedisCounter(pool, 3, 1)
	filter.KeyFunc = func(req *SMSReq, i int) string {
		return req.PhoneNumbers[i] + strconv.FormatInt(time.Now().Unix(), 10)
	}
	ctx := &Context{
		Logger: zap.NewJSON(),
	}
	ctx.Logger.SetLevel(zap.DebugLevel)

	sucCount := 0
	for i := 0; i < 5; i++ {
		req := &SMSReq{
			PhoneNumbers: []string{"1234"},
		}
		_, failed := filter.Filter(ctx, req)
		if len(failed) == 0 {
			sucCount++
		}
	}

	assert.Equal(t, 3, sucCount)
}

func TestRateLimitFilterRedisCounter_Concurrence(t *testing.T) {
	pool := buildTestRedisPool()
	defer pool.Close()

	filter := NewRateLimitFilterRedisCounter(pool, 3, 1)
	filter.KeyFunc = func(req *SMSReq, i int) string {
		return req.PhoneNumbers[i] + strconv.FormatInt(time.Now().Unix(), 10)
	}

	sucCount := int64(0)
	wg := &sync.WaitGroup{}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			ctx := &Context{
				Logger: zap.NewJSON().With(zap.Int("index", i)),
			}
			ctx.Logger.SetLevel(zap.DebugLevel)

			for i := 0; i < 10; i++ {
				req := &SMSReq{
					PhoneNumbers: []string{"1235"},
				}
				_, failed := filter.Filter(ctx, req)
				if len(failed) == 0 {
					atomic.AddInt64(&sucCount, 1)
				}
			}
		}(i)
	}
	wg.Wait()

	assert.Equal(t, int64(3), sucCount)
}

// 53577 ns/op
func BenchmarkRateLimitFilterRedisCounter_Filter(b *testing.B) {
	pool := buildTestRedisPool()
	defer pool.Close()

	filter := NewRateLimitFilterRedisCounter(pool, 3, 1)
	filter.KeyFunc = func(req *SMSReq, i int) string {
		return req.PhoneNumbers[i] + strconv.FormatInt(time.Now().Unix(), 10)
	}

	ctx := &Context{
		Logger: zap.NewJSON(),
	}
	ctx.Logger.SetLevel(zap.InfoLevel)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := &SMSReq{
			PhoneNumbers: []string{"1236"},
		}
		filter.Filter(ctx, req)
	}
}

func TestContentFilter_Filter(t *testing.T) {
	filter := &ContentFilter{}
	temp, err := buildTestTemplate()
	if err != nil {
		t.Fatal(err)
	}

	RegisterTemplate(temp.TempID, temp)
	defer RegisterTemplate(temp.TempID, nil)

	ctx := &Context{
		Logger: zap.NewJSON(),
	}
	ctx.Logger.SetLevel(zap.DebugLevel)
	content, err := filter.Filter(temp.TempID, []string{"1234", "abc"})

	assert.Equal(t, "[1234] XX验证码，30分钟内有效【abc】", content)
}

func BenchmarkContentFilter_Filter(b *testing.B) {
	filter := &ContentFilter{}
	temp, err := buildTestTemplate()
	if err != nil {
		b.Fatal(err)
	}

	RegisterTemplate(temp.TempID, temp)
	defer RegisterTemplate(temp.TempID, nil)

	ctx := &Context{
		Logger: zap.NewJSON(),
	}
	ctx.Logger.SetLevel(zap.DebugLevel)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		filter.Filter(temp.TempID, []string{"1234", "abc"})
	}
}
