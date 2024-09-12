package randomizer

import "testing"

func BenchmarkGenerateRandomStringURLSafe(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := GenerateRandomStringURLSafe(11); err != nil {
			return
		}
	}

}
