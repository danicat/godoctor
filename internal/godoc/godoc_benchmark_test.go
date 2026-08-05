package godoc

import (
	"context"
	"testing"
)

func BenchmarkLoad(b *testing.B) {
	ctx := context.Background()
	pkgPath := "os/exec"
	symbol := "Cmd"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Load(ctx, pkgPath, symbol)
		if err != nil {
			b.Fatal(err)
		}
	}
}
