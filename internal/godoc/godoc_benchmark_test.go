package godoc

import (
	"context"
	"testing"
)

func BenchmarkLoad_Cached(b *testing.B) {
	ctx := context.Background()
	pkgPath := "os/exec"
	symbol := "Cmd"

	// Warm the cache
	_, err := LoadWithFallback(ctx, pkgPath, symbol)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := LoadWithFallback(ctx, pkgPath, symbol)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkLoad_Uncached(b *testing.B) {
	ctx := context.Background()
	pkgPath := "os/exec"
	symbol := "Cmd"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		clearCacheForTest()
		b.StartTimer()

		_, err := LoadWithFallback(ctx, pkgPath, symbol)
		if err != nil {
			b.Fatal(err)
		}
	}
}
