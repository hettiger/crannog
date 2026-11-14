package ui

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fake answers questions in tests and records them.
type Fake struct {
	Terminal bool
	Answer   bool
	Choice   string
	Asked    []string
}

func (f *Fake) Interactive() bool { return f.Terminal }

func (f *Fake) Ask(q string) (bool, error) {
	f.Asked = append(f.Asked, q)
	return f.Answer, nil
}

func (f *Fake) Pick(title string, options []string) (string, error) {
	f.Asked = append(f.Asked, title)
	return f.Choice, nil
}

func TestAskConfirmation(t *testing.T) {
	f := &Fake{}
	u := &UI{Prompter: f, Out: io.Discard}
	assert.ErrorContains(t, u.AskConfirmation("Destroy?"), "pass --yes")

	u.Yes = true
	assert.NoError(t, u.AskConfirmation("Destroy?"))
	assert.Empty(t, f.Asked, "--yes does not ask")

	u.Yes = false
	f.Terminal = true
	assert.ErrorIs(t, u.AskConfirmation("Destroy?"), ErrDeclined)
	f.Answer = true
	assert.NoError(t, u.AskConfirmation("Destroy?"))
	assert.Len(t, f.Asked, 2)
}

func TestAskSetupApproval(t *testing.T) {
	f := &Fake{Answer: true}
	u := &UI{Prompter: f, Out: io.Discard, Yes: true}
	assert.ErrorContains(t, u.AskSetupApproval("Approve?"), "--yes does not approve", "--yes never approves")

	f.Terminal = true
	require.NoError(t, u.AskSetupApproval("Approve?"))
	assert.Equal(t, []string{"Approve?"}, f.Asked, "asked despite --yes")
	f.Answer = false
	assert.ErrorIs(t, u.AskSetupApproval("Approve?"), ErrDeclined)
}

func TestPickOne(t *testing.T) {
	f := &Fake{Choice: "b"}
	u := &UI{Prompter: f, Out: io.Discard}
	_, err := u.PickOne("Preset", "preset name", []string{"a", "b"})
	assert.ErrorContains(t, err, "preset name is missing")
	f.Terminal = true
	v, err := u.PickOne("Preset", "preset name", []string{"a", "b"})
	require.NoError(t, err)
	assert.Equal(t, "b", v)
}
