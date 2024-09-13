package keygen

import "testing"

func BenchmarkRandomStringGenerator_Generate(b *testing.B) {
	b.ResetTimer()

	keygen := NewRandomStringGenerator()
	for i := 0; i < b.N; i++ {
		if _, err := keygen.Generate(11); err != nil {
			return
		}
	}

}

func BenchmarkURLSafeRandomStringGenerator_Generate(b *testing.B) {
	b.ResetTimer()

	keygen := NewURLSafeRandomStringGenerator()
	for i := 0; i < b.N; i++ {
		if _, err := keygen.Generate(11); err != nil {
			return
		}
	}

}
