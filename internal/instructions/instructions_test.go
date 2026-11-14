package instructions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpdate(t *testing.T) {
	b := Begin + "\nnew\n" + End
	tests := map[string]string{
		"":         b + "\n",
		"mine":     "mine\n\n" + b + "\n",
		"mine\n":   "mine\n\n" + b + "\n",
		"mine\n\n": "mine\n\n" + b + "\n",
		"a\n" + Begin + "\nold\n" + End + "\nb\n": "a\n" + b + "\nb\n",
		Begin + "\nold\n" + End:                   b,
		// An unterminated block is left alone, a new one is appended.
		"a\n" + Begin + "\nold\n": "a\n" + Begin + "\nold\n\n" + b + "\n",
	}
	for in, want := range tests {
		assert.Equal(t, want, Update(in, b), "%q", in)
	}
	assert.Equal(t, Update("x", b), Update(Update("x", b), b), "idempotent")
}

func TestBlockIsMarked(t *testing.T) {
	assert.True(t, strings.HasPrefix(ManagedBlock, Begin+"\n"))
	assert.True(t, strings.HasSuffix(ManagedBlock, "\n"+End))
	assert.Equal(t, 1, strings.Count(ManagedBlock, Begin))
}
